package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Send/receive timing configuration and message size limits.
const (
	writeWait        = 10 * time.Second
	pongWait         = 60 * time.Second
	pingPeriod       = (pongWait * 9) / 10
	maxMsgSize       = 1 << 12 // 4 KB
	defaultInterval  = 1 * time.Second
	maxInterval      = 10 * time.Second
	maxIntervalMilli = 10_000 // 10s in ms
)

// Envelope used for WebSocket messages.
// If this type already exists in this package, keep a single definition.
type wsEnvelope struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// Upgrader for HTTP -> WebSocket. Consider tightening CheckOrigin in production.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // TODO: restrict origins for production
}

func (h *Handler) wsConnect(c *gin.Context) {
	interval := h.parseInterval(c)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		if h.log != nil {
			h.log.Errorw("ws_upgrade_failed", "err", err)
		}
		return
	}
	defer func() { _ = conn.Close() }()

	// Configure read limits and pong handler to extend read deadline.
	conn.SetReadLimit(maxMsgSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	// Reader goroutine to handle control frames and detect disconnects.
	done := make(chan struct{})
	go h.startReader(conn, done)

	// Prepare periodic writers: state updates and pings.
	ticker := time.NewTicker(interval)
	ping := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		ping.Stop()
	}()

	// Send initial state immediately.
	if err := h.sendState(c.Request.Context(), conn); err != nil {
		// If initial send fails, log and close the connection.
		if h.log != nil {
			h.log.Infow("ws_write_failed_initial", "err", err)
		}
		return
	}

	// Writer/select loop.
	for {
		select {
		case <-done:
			return
		case <-c.Request.Context().Done():
			return
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				if h.log != nil {
					h.log.Infow("ws_ping_failed", "err", err)
				}
				return
			}
		case <-ticker.C:
			if err := h.sendState(c.Request.Context(), conn); err != nil {
				// Log and keep the loop only for transient write errors; close on hard errors.
				if h.log != nil {
					h.log.Infow("ws_write_failed", "err", err)
				}
				return
			}
		}
	}
}

// ... existing code ...
// Helper: parseInterval reads ?interval=2s or ?interval_ms=2000 with bounds.
func (h *Handler) parseInterval(c *gin.Context) time.Duration {
	interval := defaultInterval

	if s := c.Query("interval"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 && d <= maxInterval {
			return d
		}
	}

	if ms := c.Query("interval_ms"); ms != "" {
		if v, err := strconv.Atoi(ms); err == nil && v > 0 && v <= maxIntervalMilli {
			return time.Duration(v) * time.Millisecond
		}
	}

	return interval
}

// Helper: startReader drains incoming messages to handle control frames and detect closure.
func (h *Handler) startReader(conn *websocket.Conn, done chan<- struct{}) {
	defer close(done)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if h.log != nil {
				h.log.Infow("ws_read_closed", "err", err)
			}
			return
		}
	}
}

// Helper: sendState fetches and writes the current state with a write deadline.
func (h *Handler) sendState(ctx context.Context, conn *websocket.Conn) error {
	st, err := h.services.Monitoring.GetState(ctx)
	if err != nil {
		if h.log != nil {
			h.log.Errorw("ws_get_state_failed", "err", err)
		}
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteJSON(wsEnvelope{Type: "state", Data: st})
}
