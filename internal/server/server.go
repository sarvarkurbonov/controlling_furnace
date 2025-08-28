package server

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Server wraps an *http.Server to provide start/shutdown lifecycle.
type Server struct {
	httpServer *http.Server
}

// Extracted constants to avoid magic numbers and centralize tuning knobs.
const (
	maxHeaderBytes    = 1 << 20 // 1 MB
	readHeaderTimeout = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

// newHTTPServer builds a configured *http.Server for the given address and handler.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		MaxHeaderBytes:    maxHeaderBytes,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// normalizeAddr ensures the provided port is a valid address (accepts "8080" or ":8080").
func normalizeAddr(port string) string {
	if port == "" {
		// Leave defaulting to callers (e.g., runHTTPServer), to avoid duplicating policy here.
		return ""
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

// Run starts the HTTP server on the given port using the provided handler.
func (s *Server) Run(port string, handler http.Handler) error {
	addr := normalizeAddr(port)
	// ... existing code ...
	s.httpServer = newHTTPServer(addr, handler)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server, allowing in-flight requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	// ... existing code ...
	return s.httpServer.Shutdown(ctx)
}
