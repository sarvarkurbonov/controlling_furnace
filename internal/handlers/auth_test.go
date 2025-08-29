package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"controlling_furnace/internal/service"
)

func TestAuthHandlers_SignUpAndSignIn(t *testing.T) {
	auth := &mockAuth{signUpID: 42, genTokenToken: "tok123", parseID: 1}
	s := &service.Service{Authorization: auth}
	r := newTestRouter(s)

	// sign-up success
	body := bytes.NewBufferString(`{"username":"u","password":"p"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-up status=%d, body=%s", w.Code, w.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	if int(m["id"].(float64)) != 42 {
		t.Fatalf("expected id=42, got %v", m["id"])
	}

	// sign-in success
	body = bytes.NewBufferString(`{"username":"u","password":"p"}`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/sign-in", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sign-in status=%d, body=%s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	if m["token"] != "tok123" {
		t.Fatalf("expected token tok123, got %v", m["token"])
	}

	// sign-in invalid body → 400
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/sign-in", bytes.NewBufferString(`{"username":1}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", w.Code)
	}
}
