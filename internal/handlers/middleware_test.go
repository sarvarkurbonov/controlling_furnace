package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"controlling_furnace/internal/service"

	"github.com/gin-gonic/gin"
)

// minimal router wiring only the middleware + a protected endpoint
func newMiddlewareOnlyRouter(s *service.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(s, nil)
	r.GET("/secure", h.userIdMiddleware, func(c *gin.Context) {
		uid, _ := c.Get("userId")
		c.JSON(http.StatusOK, gin.H{"ok": true, "userId": uid})
	})
	return r
}

func TestUserIDMiddleware_Errors(t *testing.T) {
	type want struct {
		code   int
		errMsg string
	}
	cases := []struct {
		name   string
		header string
		want   want
	}{
		{
			name:   "missing header",
			header: "",
			want:   want{code: http.StatusUnauthorized, errMsg: "missing Authorization header"},
		},
		{
			name:   "invalid scheme",
			header: "Token abc",
			// actual implementation returns "invalid Authorization header format"
			want: want{code: http.StatusUnauthorized, errMsg: "invalid Authorization header format"},
		},
		{
			name:   "bearer without token",
			header: "Bearer",
			want:   want{code: http.StatusUnauthorized, errMsg: "invalid Authorization header format"},
		},
		{
			name:   "expired/invalid token",
			header: "Bearer expired",
			want:   want{code: http.StatusUnauthorized, errMsg: "invalid or expired token"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &mockAuth{parseErr: nil}
			if tc.name == "expired/invalid token" {
				auth.parseErr = errors.New("expired")
			}
			s := &service.Service{Authorization: auth}
			r := newMiddlewareOnlyRouter(s)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/secure", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			r.ServeHTTP(w, req)

			if w.Code != tc.want.code {
				t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.want.code, w.Body.String())
			}

			var out struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if out.Error != tc.want.errMsg {
				t.Fatalf("error message: got %q, want %q", out.Error, tc.want.errMsg)
			}
		})
	}
}

func TestUserIDMiddleware_SuccessSetsUserIDAndProceeds(t *testing.T) {
	auth := &mockAuth{parseID: 123, parseErr: nil}
	s := &service.Service{Authorization: auth}
	r := newMiddlewareOnlyRouter(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		OK     bool `json:"ok"`
		UserID int  `json:"userId"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK || resp.UserID != 123 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if auth.lastParseToken != "good-token" {
		t.Fatalf("ParseToken got %q, want %q", auth.lastParseToken, "good-token")
	}
}
