package service

import (
	"context"
	"time"

	"controlling_furnace"
	"controlling_furnace/internal/repository"

	"github.com/google/uuid"
)

// ----------- Simulation constants -----------
const (
	AmbientC          = 25.0   // ambient temperature °C
	MaxSafeC          = 1000.0 // overheat threshold °C
	RampUpCPerSec     = 3.0    // °C per second when HEAT
	RampDownCPerSec   = 5.0    // °C per second when COOL
	StandbyCoolPerSec = 0.5    // °C per second cooling drift in STANDBY
	SoakToleranceC    = 2.0    // °C band for "at target"
)

// Modes
const (
	ModeHeat    = "HEAT"
	ModeCool    = "COOL"
	ModeStandby = "STANDBY"
)

// SimulatorService updates furnace state over time.
type SimulatorService struct {
	stateRepo repository.StateRepo
	eventRepo repository.EventRepo
}

// NewSimulatorService returns a simulator with defaults.
func NewSimulatorService(stateRepo repository.StateRepo, eventRepo repository.EventRepo) *SimulatorService {
	return &SimulatorService{
		stateRepo: stateRepo,
		eventRepo: eventRepo,
	}
}

// Run ticks at the given interval until ctx is canceled.
func (s *SimulatorService) Run(ctx context.Context, tick time.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			st, err := s.stateRepo.Load(ctx)
			if err != nil {
				continue
			}
			// Initialize state if empty
			if st.ID == 0 {
				st = controlling_furnace.FurnaceState{
					ID:           1,
					Mode:         ModeStandby,
					CurrentTempC: AmbientC,
					IsRunning:    false,
					UpdatedAt:    now.UTC(),
				}
				_ = s.stateRepo.Save(ctx, st)
				continue
			}
			// time passed since last update
			elapsed := now.Sub(st.UpdatedAt).Seconds()
			if elapsed < 1 {
				// less than 1s → skip until more time passes
				continue
			}

			changed := false

			// If not running → drift to ambient
			if !st.IsRunning {
				if s.driftToAmbient(&st, elapsed) {
					changed = true
				}
				if changed {
					st.UpdatedAt = now.UTC()
					_ = s.stateRepo.Save(ctx, st)
				}
				continue
			}

			// --------------------
			// When running
			// --------------------
			switch st.Mode {
			case ModeHeat:
				if s.handleHeat(ctx, &st, elapsed, now) {
					changed = true
				}
			case ModeCool:
				if s.handleCooling(&st, elapsed, RampDownCPerSec) {
					changed = true
				}
			case ModeStandby:
				if s.handleCooling(&st, elapsed, StandbyCoolPerSec) {
					changed = true
				}
			default:
				// unknown mode → treat like standby
				if s.handleCooling(&st, elapsed, StandbyCoolPerSec) {
					changed = true
				}
			}

			// Overheat detection
			if s.detectAndLogOverheat(ctx, &st, now) {
				changed = true
			}

			if changed {
				st.UpdatedAt = now.UTC()
				_ = s.stateRepo.Save(ctx, st)
			}
		}
	}
}

// ... existing code ...

// driftToAmbient cools toward ambient when not running. Returns true if temp changed.
func (s *SimulatorService) driftToAmbient(st *controlling_furnace.FurnaceState, elapsed float64) bool {
	if st.CurrentTempC > AmbientC {
		st.CurrentTempC = maxFloat(st.CurrentTempC-StandbyCoolPerSec*elapsed, AmbientC)
		return true
	}
	return false
}

// handleHeat advances temperature toward target and decrements soak timer.
// May switch to COOL and append an event. Returns true if state changed.
func (s *SimulatorService) handleHeat(ctx context.Context, st *controlling_furnace.FurnaceState, elapsed float64, now time.Time) bool {
	changed := false
	tempChanged := false
	soakElapsed := 0.0

	prevTemp := st.CurrentTempC

	// Ramp up if below (target - tolerance)
	if prevTemp < st.TargetTempC-SoakToleranceC {
		// Compute new temperature and time to reach target based on previous temp.
		timeToTarget := (st.TargetTempC - prevTemp) / RampUpCPerSec
		if timeToTarget < 0 {
			timeToTarget = 0
		}
		st.CurrentTempC = prevTemp + RampUpCPerSec*elapsed
		if st.CurrentTempC > st.TargetTempC {
			st.CurrentTempC = st.TargetTempC
		}
		tempChanged = st.CurrentTempC != prevTemp

		// If we reached target within this tick, part of the tick (after reaching target) is soak time.
		if timeToTarget < elapsed {
			soakElapsed = elapsed - timeToTarget
		}
	} else {
		// Already at/near target → entire tick is soak time
		soakElapsed = elapsed
		// Clamp overshoot
		if st.CurrentTempC > st.TargetTempC {
			st.CurrentTempC = st.TargetTempC
			tempChanged = true
		}
	}

	// Countdown during soak
	if st.RemainingSeconds > 0 && soakElapsed > 0 {
		dec := int(soakElapsed) // whole seconds at/near target
		if dec >= 1 {
			if st.RemainingSeconds > dec {
				st.RemainingSeconds -= dec
			} else {
				st.RemainingSeconds = 0
				st.Mode = ModeCool
				_ = s.eventRepo.Append(ctx, controlling_furnace.FurnaceEvent{
					EventID:     uuid.NewString(),
					OccurredAt:  now.UTC(),
					Type:        "MODE_CHANGE",
					Description: "Duration elapsed; switched to COOL",
					Metadata:    map[string]any{"from": ModeHeat, "to": ModeCool},
				})
			}
			changed = true
		}
	}

	if tempChanged {
		changed = true
	}

	return changed
}

// handleCooling cools toward ambient by a given rate. Returns true if temp changed.
func (s *SimulatorService) handleCooling(st *controlling_furnace.FurnaceState, elapsed float64, ratePerSec float64) bool {
	if st.CurrentTempC > AmbientC {
		st.CurrentTempC = maxFloat(st.CurrentTempC-ratePerSec*elapsed, AmbientC)
		return true
	}
	return false
}

// detectAndLogOverheat appends an error event and sets error code if needed.
// Returns true if state changed (error code added).
func (s *SimulatorService) detectAndLogOverheat(ctx context.Context, st *controlling_furnace.FurnaceState, now time.Time) bool {
	stateChanged := false
	if st.CurrentTempC > MaxSafeC {
		if !hasString(st.ErrorCodes, "OVERHEAT") {
			st.ErrorCodes = append(st.ErrorCodes, "OVERHEAT")
			stateChanged = true
		}
		_ = s.eventRepo.Append(ctx, controlling_furnace.FurnaceEvent{
			EventID:     uuid.NewString(),
			OccurredAt:  now.UTC(),
			Type:        "ERROR",
			Description: "Overheat detected",
			Metadata: map[string]any{
				"temp_c":    st.CurrentTempC,
				"max_safe":  MaxSafeC,
				"mode":      st.Mode,
				"isRunning": st.IsRunning,
			},
		})
	}
	return stateChanged
}

// helpers
func maxFloat(a, b float64) float64 {
	if a >= b {
		return a
	}
	return b
}

func hasString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
