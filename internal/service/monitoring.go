package service

import (
	"context"
	"time"

	"controlling_furnace"
	"controlling_furnace/internal/repository"
)

const (
	modeStandby         = "STANDBY"
	defaultAmbientTempC = 25.0
)

// ... existing code ...
type MonitoringService struct {
	stateRepo repository.StateRepo
}

func NewMonitoringService(stateRepo repository.StateRepo) *MonitoringService {
	return &MonitoringService{stateRepo: stateRepo}
}

// GetState returns the latest persisted furnace state.
// If no state is persisted yet, returns a baseline STANDBY snapshot.
func (s *MonitoringService) GetState(ctx context.Context) (controlling_furnace.FurnaceState, error) {
	state, err := s.stateRepo.Load(ctx)
	if err != nil {
		return controlling_furnace.FurnaceState{}, err
	}
	if state.ID == 0 {
		return s.baselineState(), nil
	}
	state.UpdatedAt = toUTC(state.UpdatedAt)
	return state, nil
}

// ... existing code ...

// baselineState returns a sensible default snapshot for an uninitialized DB.
func (s *MonitoringService) baselineState() controlling_furnace.FurnaceState {
	return controlling_furnace.FurnaceState{
		ID:               1, // DB schema enforces single-row state with id=1
		Mode:             modeStandby,
		CurrentTempC:     defaultAmbientTempC,
		TargetTempC:      0,
		RemainingSeconds: 0,
		ErrorCodes:       nil,
		IsRunning:        false,
		UpdatedAt:        time.Now().UTC(),
	}
}

// toUTC normalizes non-zero time to UTC, preserving zero values.
func toUTC(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}
