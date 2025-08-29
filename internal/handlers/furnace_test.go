package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"controlling_furnace/internal/models"
	"controlling_furnace/internal/service"
)

func TestFurnaceHandlers_StartStopSetMode_GetState(t *testing.T) {
	auth := &mockAuth{parseID: 7}
	mon := &mockMonitoring{state: models.FurnaceState{Mode: "HEAT", CurrentTempC: 500, RemainingSeconds: 10}}
	fu := &mockFurnace{}
	s := &service.Service{
		Authorization: auth,
		Monitoring:    mon,
		Furnace:       fu,
	}
	r := newTestRouter(s)

	// GET state requires auth → 401 without header
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/furnace/state", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}

	// With auth → 200 and state body
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/furnace/state", nil)
	for k, vv := range authHeader("valid") {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("state status=%d, body=%s", w.Code, w.Body.String())
	}
	var st models.FurnaceState
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if st.Mode != "HEAT" || st.CurrentTempC != 500 {
		t.Fatalf("unexpected state: %+v", st)
	}

	// POST /start → 200, calls Furnace.Start and includes state
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/furnace/start", nil)
	for k, vv := range authHeader("valid") {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start status=%d, body=%s", w.Code, w.Body.String())
	}
	if fu.startCalled != 1 {
		t.Fatalf("expected Start to be called once, got %d", fu.startCalled)
	}
	var resp struct {
		Status string              `json:"status"`
		State  models.FurnaceState `json:"state"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Status != statusStarted {
		t.Fatalf("expected status %q, got %q", statusStarted, resp.Status)
	}
	if resp.State.Mode != "HEAT" {
		t.Fatalf("state missing/invalid in response: %+v", resp.State)
	}

	// POST /mode → 200, passes parameters and includes mode
	body := bytes.NewBufferString(`{"mode":"HEAT","target_temp_c":800,"duration_sec":600}`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/furnace/mode", body)
	req.Header.Set("Content-Type", "application/json")
	for k, vv := range authHeader("valid") {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mode status=%d, body=%s", w.Code, w.Body.String())
	}
	if fu.setModeCalls != 1 {
		t.Fatalf("SetMode calls=%d", fu.setModeCalls)
	}
	if fu.lastSetMode.Mode != "HEAT" || fu.lastSetMode.TargetTempC != 800 || fu.lastSetMode.DurationSec != 600 {
		t.Fatalf("wrong SetMode params: %+v", fu.lastSetMode)
	}
	var modeResp struct {
		Status string              `json:"status"`
		Mode   string              `json:"mode"`
		State  models.FurnaceState `json:"state"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &modeResp)
	if modeResp.Status != statusModeSet || modeResp.Mode != "HEAT" {
		t.Fatalf("bad mode response: %+v", modeResp)
	}

	// POST /stop → 200 and Start/Stop counters
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/furnace/stop", nil)
	for k, vv := range authHeader("valid") {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stop status=%d, body=%s", w.Code, w.Body.String())
	}
	if fu.stopCalled != 1 {
		t.Fatalf("expected Stop to be called once, got %d", fu.stopCalled)
	}
}
