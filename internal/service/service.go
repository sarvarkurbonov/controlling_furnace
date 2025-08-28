package service

import (
	"context"
	"controlling_furnace"
	"time"

	// uses your FurnaceState / FurnaceEvent structs
	"controlling_furnace/internal/repository"
)

type Authorization interface {
	SignUp(username, password string) (int, error)
	GenerateToken(username, password string) (string, error)
	ParseToken(accessToken string) (int, error)
}

// Furnace exposes control operations: start/stop and mode changes.
type Furnace interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SetMode(ctx context.Context, p ModeParams) error
}

// Monitoring exposes read-only state (temperature, mode, remaining, errors).
type Monitoring interface {
	GetState(ctx context.Context) (controlling_furnace.FurnaceState, error)
}

// EventLog exposes append-only logs with filtering access.
type EventLog interface {
	List(ctx context.Context, f LogFilter) ([]controlling_furnace.FurnaceEvent, error)
}

// Simulator runs the background loop that updates temperature/remaining time.
// Stop via context cancellation in main() for graceful shutdown.
type Simulator interface {
	Run(ctx context.Context, tick time.Duration)
}

//
// Root Service aggregates all sub-services (style like your Todo example).
//

type Service struct {
	Furnace
	Monitoring
	EventLog
	Simulator
	Authorization
}

// NewService wires repository layer into concrete services (same style as your Todo `NewService`).
// You will implement NewFurnaceService/NewMonitoringService/NewEventLogService/NewSimulatorService
// in their own files, taking the repo deps you define under internal/repository.
func NewService(repos *repository.Repository) *Service {
	return &Service{
		Furnace:       NewFurnaceService(repos.StateRepo, repos.EventRepo),
		Monitoring:    NewMonitoringService(repos.StateRepo),
		EventLog:      NewEventLogService(repos.EventRepo),
		Simulator:     NewSimulatorService(repos.StateRepo, repos.EventRepo),
		Authorization: NewAuthService(repos.Auth),
	}
}
