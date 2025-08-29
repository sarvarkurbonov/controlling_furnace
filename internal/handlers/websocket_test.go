// internal/handlers/websocket_handlers_test.go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"controlling_furnace/internal/models"
	"controlling_furnace/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// --- parseInterval unit tests ---

func TestParseInterval(t *testing.T) {
	h := NewHandler(&service.Service{}, nil)

	cases := []struct {
		name string
		u    string
		want time.Duration
	}{
		{"default_when_missing", "/ws", 1 * time.Second},
		{"interval_string_valid", "/ws?interval=200ms", 200 * time.Millisecond},
		{"interval_ms_valid", "/ws?interval_ms=150", 150 * time.Millisecond},
		{"interval_too_large", "/ws?interval=20s", 1 * time.Second},
		{"interval_ms_too_large", "/ws?interval_ms=20000", 1 * time.Second},
		{"interval_invalid_string", "/ws?interval=bogus", 1 * time.Second},
		{"interval_ms_invalid", "/ws?interval_ms=NaN", 1 * time.Second},
		{"both_present_interval_wins", "/ws?interval=2s&interval_ms=150", 2 * time.Second},
		{"both_present_invalid_interval_ms_used", "/ws?interval=bogus&interval_ms=250", 250 * time.Millisecond},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.u, nil)
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			got := h.parseInterval(c)
			if got != tc.want {
				t.Fatalf("got %v, want %v for %s", got, tc.want, tc.u)
			}
		})
	}
}

// --- websocket integration tests ---

func TestWebSocket_StateStream_InitialAndPeriodic(t *testing.T) {
	// Mock monitoring returns a fixed state
	mon := &mockMonitoring{state: models.FurnaceState{
		Mode:             "HEAT",
		CurrentTempC:     700,
		TargetTempC:      800,
		RemainingSeconds: 60,
		IsRunning:        true,
	}}
	s := &service.Service{Monitoring: mon}

	// Build router with /ws
	r := gin.New()
	h := NewHandler(s, nil)
	r.GET("/ws", h.wsConnect)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Build ws URL
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/ws"
	q := u.Query()
	q.Set("interval_ms", "20") // fast ticks for the test
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	type envelope struct {
		Type  string          `json:"type"`
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}

	// Read initial state
	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var env envelope
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatalf("read initial: %v", err)
	}
	if env.Type != "state" || len(env.Data) == 0 {
		t.Fatalf("bad envelope: %+v", env)
	}
	var st models.FurnaceState
	if err := json.Unmarshal(env.Data, &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if st.Mode != "HEAT" || st.CurrentTempC != 700 || !st.IsRunning {
		t.Fatalf("unexpected state: %+v", st)
	}

	// Read a subsequent tick
	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	env = envelope{}
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatalf("read second: %v", err)
	}
	if env.Type != "state" {
		t.Fatalf("expected type=state, got %+v", env)
	}
}

func TestWebSocket_InitialGetStateError_Closes(t *testing.T) {
	mon := &mockMonitoring{err: errors.New("boom")}
	s := &service.Service{Monitoring: mon}

	r := gin.New()
	h := NewHandler(s, nil)
	r.GET("/ws", h.wsConnect)

	srv := httptest.NewServer(r)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/ws"
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	// The server should close immediately after failing initial GetState/WriteJSON
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var raw json.RawMessage
	if err := conn.ReadJSON(&raw); err == nil {
		t.Fatalf("expected read error (closed), got message: %s", string(raw))
	}
}
