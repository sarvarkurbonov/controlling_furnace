package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"controlling_furnace" // your domain models: FurnaceState, FurnaceEvent
	"controlling_furnace/internal/repository"

	"github.com/google/uuid"
)

// -------- Public API --------

// -------- Implementation --------

type FurnaceService struct {
	stateRepo repository.StateRepo
	eventRepo repository.EventRepo
}

func NewFurnaceService(stateRepo repository.StateRepo, eventRepo repository.EventRepo) *FurnaceService {
	return &FurnaceService{stateRepo: stateRepo, eventRepo: eventRepo}
}

var (
	errInvalidMode    = errors.New("invalid mode: must be HEAT, COOL, or STANDBY")
	errInvalidHeatCfg = errors.New("invalid HEAT params: target_temp_c > 0 and duration_sec > 0 are required")
)

// Start sets IsRunning=true and logs START.
// If state row doesn't exist yet, it initializes a default one.
func (s *FurnaceService) Start(ctx context.Context) error {
	now := time.Now().UTC()

	st, err := s.stateRepo.Load(ctx)
	if err != nil {
		return err
	}
	// Initialize default state if empty
	if st.ID == 0 {
		st = controlling_furnace.FurnaceState{
			ID:               1,
			Mode:             "STANDBY",
			CurrentTempC:     25, // ambient default
			TargetTempC:      0,
			RemainingSeconds: 0,
			ErrorCodes:       nil,
			IsRunning:        true,
			UpdatedAt:        now,
		}
	} else {
		st.IsRunning = true
		st.UpdatedAt = now
	}

	if err := s.stateRepo.Save(ctx, st); err != nil {
		return err
	}

	return s.eventRepo.Append(ctx, controlling_furnace.FurnaceEvent{
		EventID:     uuid.NewString(),
		OccurredAt:  now,
		Type:        "START",
		Description: "Furnace started",
	})
}

// Stop sets IsRunning=false, switches to STANDBY, clears timing/target, and logs STOP.
func (s *FurnaceService) Stop(ctx context.Context) error {
	now := time.Now().UTC()

	st, err := s.stateRepo.Load(ctx)
	if err != nil {
		return err
	}
	if st.ID == 0 {
		// If no state existed, create a baseline stopped state.
		st.ID = 1
	}
	st.IsRunning = false
	st.Mode = "STANDBY"
	st.TargetTempC = 0
	st.RemainingSeconds = 0
	st.UpdatedAt = now

	if err := s.stateRepo.Save(ctx, st); err != nil {
		return err
	}

	return s.eventRepo.Append(ctx, controlling_furnace.FurnaceEvent{
		EventID:     uuid.NewString(),
		OccurredAt:  now,
		Type:        "STOP",
		Description: "Furnace stopped",
	})
}

// SetMode updates the current mode.
// - HEAT requires target_temp_c > 0 and duration_sec > 0.
// - COOL/STANDBY clear target/duration.
// This does NOT implicitly start/stop the furnace; Start/Stop own IsRunning.
func (s *FurnaceService) SetMode(ctx context.Context, p ModeParams) error {
	now := time.Now().UTC()

	// Basic validation
	switch p.Mode {
	case "HEAT":
		if !(p.TargetTempC > 0 && p.DurationSec > 0) {
			return errInvalidHeatCfg
		}
		if p.TargetTempC < AmbientC {
			return fmt.Errorf("target temperature %.1f is below ambient temperature %.1f", p.TargetTempC, AmbientC)
		}
		if p.TargetTempC > MaxSafeC { // ✅ new check
			return fmt.Errorf("target temperature %.1f exceeds max safe limit %.1f", p.TargetTempC, MaxSafeC)
		}

	case "COOL", "STANDBY":
		// ok
	default:
		return errInvalidMode
	}

	st, err := s.stateRepo.Load(ctx)
	if err != nil {
		return err
	}
	if st.ID == 0 {
		// Furnace never started
		return errors.New("cannot change mode: furnace is not running, start it first")
	}

	if !st.IsRunning {
		return errors.New("cannot change mode: furnace is stopped, start it first")
	}

	// Apply mode change
	st.Mode = p.Mode
	if p.Mode == "HEAT" {
		st.TargetTempC = p.TargetTempC
		st.RemainingSeconds = p.DurationSec
	} else {
		st.TargetTempC = 0
		st.RemainingSeconds = 0
	}
	st.UpdatedAt = now

	if err := s.stateRepo.Save(ctx, st); err != nil {
		return err
	}

	return s.eventRepo.Append(ctx, controlling_furnace.FurnaceEvent{
		EventID:     uuid.NewString(),
		OccurredAt:  now,
		Type:        "MODE_CHANGE",
		Description: "Mode changed to " + p.Mode,
		Metadata: map[string]any{
			"target_temp_c": st.TargetTempC,
			"duration_sec":  st.RemainingSeconds,
			"is_running":    st.IsRunning,
		},
	})
}
