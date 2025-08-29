package handlers

import (
	"context"
	"net/http"
	"time"

	"controlling_furnace/internal/models"
	"controlling_furnace/internal/service"

	"github.com/gin-gonic/gin"
)

// ---- Service Mocks ----

type mockAuth struct {
	signUpID      int
	signUpErr     error
	genTokenToken string
	genTokenErr   error
	parseID       int
	parseErr      error

	lastSignUpUsername string
	lastSignUpPassword string
	lastGenUsername    string
	lastGenPassword    string
	lastParseToken     string
}

func (m *mockAuth) SignUp(username, password string) (int, error) {
	m.lastSignUpUsername = username
	m.lastSignUpPassword = password
	return m.signUpID, m.signUpErr
}
func (m *mockAuth) GenerateToken(username, password string) (string, error) {
	m.lastGenUsername = username
	m.lastGenPassword = password
	return m.genTokenToken, m.genTokenErr
}
func (m *mockAuth) ParseToken(token string) (int, error) {
	m.lastParseToken = token
	return m.parseID, m.parseErr
}

type mockFurnace struct {
	startErr     error
	stopErr      error
	setModeErr   error
	lastSetMode  service.ModeParams
	startCalled  int
	stopCalled   int
	setModeCalls int
}

func (m *mockFurnace) Start(ctx context.Context) error {
	m.startCalled++
	return m.startErr
}
func (m *mockFurnace) Stop(ctx context.Context) error {
	m.stopCalled++
	return m.stopErr
}
func (m *mockFurnace) SetMode(ctx context.Context, p service.ModeParams) error {
	m.setModeCalls++
	m.lastSetMode = p
	return m.setModeErr
}

type mockMonitoring struct {
	state models.FurnaceState
	err   error
}

func (m *mockMonitoring) GetState(ctx context.Context) (models.FurnaceState, error) {
	return m.state, m.err
}

type mockEventLog struct {
	resp     []models.FurnaceEvent
	err      error
	lastFrom time.Time
	lastTo   time.Time
	lastType string
}

func (m *mockEventLog) List(ctx context.Context, f service.LogFilter) ([]models.FurnaceEvent, error) {
	m.lastFrom = f.From
	m.lastTo = f.To
	m.lastType = f.Type
	return m.resp, m.err
}

// ---- Shared Test Helpers ----

func newTestRouter(s *service.Service) *gin.Engine {
	h := NewHandler(s, nil)
	gin.SetMode(gin.TestMode)
	return h.InitRoutes()
}

func authHeader(token string) http.Header {
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
}
