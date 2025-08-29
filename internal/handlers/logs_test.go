package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"controlling_furnace/internal/models"
	"controlling_furnace/internal/service"
)

func TestLogsHandler_ListAndValidation(t *testing.T) {
	auth := &mockAuth{parseID: 99}
	now := time.Now().UTC().Truncate(time.Second)
	events := []models.FurnaceEvent{
		{EventID: "e1", OccurredAt: now, Type: "START", Description: "start"},
		{EventID: "e2", OccurredAt: now.Add(1 * time.Second), Type: "MODE_CHANGE", Description: "mode"},
	}
	logs := &mockEventLog{resp: events}
	s := &service.Service{
		Authorization: auth,
		EventLog:      logs,
	}
	r := newTestRouter(s)

	// Missing/invalid 'from' → 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/?from=notatime", nil)
	for k, vv := range authHeader("valid") {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid 'from', got %d", w.Code)
	}

	// Valid range and type (lowercase type should be normalized to upper in service call)
	w = httptest.NewRecorder()
	q := "/api/v1/logs/?from=" + now.Format(time.RFC3339) + "&to=" + now.Add(2*time.Second).Format(time.RFC3339) + "&type=mode_change"
	req = httptest.NewRequest(http.MethodGet, q, nil)
	for k, vv := range authHeader("valid") {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logs status=%d, body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Count  int                   `json:"count"`
		Events []models.FurnaceEvent `json:"events"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Count != 2 || len(out.Events) != 2 {
		t.Fatalf("unexpected response: %+v", out)
	}
	if logs.lastType != "MODE_CHANGE" {
		t.Fatalf("expected lastType MODE_CHANGE, got %q", logs.lastType)
	}
}
