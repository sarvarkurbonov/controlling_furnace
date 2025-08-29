package service

import (
	"context"
	"testing"
	"time"

	"controlling_furnace/internal/models"
)

// ---- Test doubles ----

// simStateRepoStub is a minimal stub for repository.StateRepo.
type simStateRepoStub struct {
	loadResp models.FurnaceState
	saves    []models.FurnaceState
}

func (s *simStateRepoStub) Save(ctx context.Context, st models.FurnaceState) error {
	s.saves = append(s.saves, st)
	return nil
}
func (s *simStateRepoStub) Load(ctx context.Context) (models.FurnaceState, error) {
	return s.loadResp, nil
}

// simEventRepoStub is a minimal stub for repository.EventRepo.
type simEventRepoStub struct {
	appends []models.FurnaceEvent
}

func (e *simEventRepoStub) Append(ctx context.Context, ev models.FurnaceEvent) error {
	e.appends = append(e.appends, ev)
	return nil
}
func (e *simEventRepoStub) List(ctx context.Context, from, to time.Time, typ string) ([]models.FurnaceEvent, error) {
	return nil, nil
}

// ---- Tests ----

func TestDriftToAmbient_CoolsTowardAmbientAndClamps(t *testing.T) {
	svc := NewSimulatorService(&simStateRepoStub{}, &simEventRepoStub{})

	st := models.FurnaceState{CurrentTempC: AmbientC + 10}
	if !svc.driftToAmbient(&st, 10) {
		t.Fatalf("expected change when above ambient")
	}
	want := AmbientC + 10 - StandbyCoolPerSec*10
	if st.CurrentTempC != want {
		t.Fatalf("got %.2f, want %.2f", st.CurrentTempC, want)
	}

	st = models.FurnaceState{CurrentTempC: AmbientC + 1}
	_ = svc.driftToAmbient(&st, 10)
	if st.CurrentTempC != AmbientC {
		t.Fatalf("expected clamp to AmbientC, got %.2f", st.CurrentTempC)
	}

	st = models.FurnaceState{CurrentTempC: AmbientC}
	if svc.driftToAmbient(&st, 5) {
		t.Fatalf("did not expect change at ambient")
	}
}

func TestHandleCooling_UsesRateAndClamps(t *testing.T) {
	svc := NewSimulatorService(&simStateRepoStub{}, &simEventRepoStub{})

	st := models.FurnaceState{CurrentTempC: 100}
	_ = svc.handleCooling(&st, 2, RampDownCPerSec)
	want := 100 - RampDownCPerSec*2
	if st.CurrentTempC != want {
		t.Fatalf("got %.2f, want %.2f", st.CurrentTempC, want)
	}

	st = models.FurnaceState{CurrentTempC: AmbientC + 1}
	_ = svc.handleCooling(&st, 60, RampDownCPerSec)
	if st.CurrentTempC != AmbientC {
		t.Fatalf("expected clamp to AmbientC, got %.2f", st.CurrentTempC)
	}

	st = models.FurnaceState{CurrentTempC: AmbientC}
	if svc.handleCooling(&st, 1, RampDownCPerSec) {
		t.Fatalf("did not expect change at ambient")
	}
}

func TestHandleHeat_RampTowardTargetAndSoakCountdown(t *testing.T) {
	ctx := context.Background()
	ev := &simEventRepoStub{}
	svc := NewSimulatorService(&simStateRepoStub{}, ev)

	t.Run("ramps up when below target", func(t *testing.T) {
		st := models.FurnaceState{Mode: ModeHeat, CurrentTempC: 100, TargetTempC: 110}
		_ = svc.handleHeat(ctx, &st, 2, time.Now())
		want := 100 + RampUpCPerSec*2
		if st.CurrentTempC != want {
			t.Fatalf("got %.2f, want %.2f", st.CurrentTempC, want)
		}
	})

	t.Run("reaches target within tick and partial soak does not decrement whole seconds", func(t *testing.T) {
		st := models.FurnaceState{
			Mode:             ModeHeat,
			CurrentTempC:     107.6, // below (target - tolerance)=108 → ramp branch
			TargetTempC:      110,
			RemainingSeconds: 5,
		}
		_ = svc.handleHeat(ctx, &st, 1, time.Now()) // timeToTarget≈0.8s; soakElapsed≈0.2s → int(0)=0
		if st.CurrentTempC != st.TargetTempC {
			t.Fatalf("should clamp to target, got %.2f", st.CurrentTempC)
		}
		if st.RemainingSeconds != 5 {
			t.Fatalf("expected unchanged RemainingSeconds, got %d", st.RemainingSeconds)
		}
	})

	t.Run("already at/near target: soak consumes whole seconds", func(t *testing.T) {
		st := models.FurnaceState{Mode: ModeHeat, CurrentTempC: 109, TargetTempC: 110, RemainingSeconds: 3}
		_ = svc.handleHeat(ctx, &st, 2.4, time.Now()) // 2 whole seconds
		if st.RemainingSeconds != 1 {
			t.Fatalf("got %d, want 1", st.RemainingSeconds)
		}
	})

	t.Run("soak reaches zero: switch to COOL and append event", func(t *testing.T) {
		ev.appends = nil
		st := models.FurnaceState{Mode: ModeHeat, CurrentTempC: 110, TargetTempC: 110, RemainingSeconds: 1}
		_ = svc.handleHeat(ctx, &st, 1.2, time.Now())
		if st.Mode != ModeCool {
			t.Fatalf("expected COOL, got %q", st.Mode)
		}
		if len(ev.appends) != 1 || ev.appends[0].Type != "MODE_CHANGE" {
			t.Fatalf("expected MODE_CHANGE event, got %+v", ev.appends)
		}
	})
}

func TestDetectAndLogOverheat_SetsErrorOnceAndAlwaysLogs(t *testing.T) {
	ctx := context.Background()
	ev := &simEventRepoStub{}
	svc := NewSimulatorService(&simStateRepoStub{}, ev)

	st := models.FurnaceState{Mode: ModeHeat, IsRunning: true, CurrentTempC: MaxSafeC + 10}
	changed := svc.detectAndLogOverheat(ctx, &st, time.Now())
	if !changed || !containsStr(st.ErrorCodes, "OVERHEAT") {
		t.Fatalf("OVERHEAT not set properly")
	}
	if len(ev.appends) != 1 || ev.appends[0].Type != "ERROR" {
		t.Fatalf("expected 1 ERROR event, got %+v", ev.appends)
	}

	// Second detection
	changed = svc.detectAndLogOverheat(ctx, &st, time.Now().Add(time.Second))
	if changed {
		t.Fatalf("did not expect state change again")
	}
	if len(ev.appends) != 2 {
		t.Fatalf("expected 2 ERROR events, got %d", len(ev.appends))
	}
}

func containsStr(ss []string, w string) bool {
	for _, s := range ss {
		if s == w {
			return true
		}
	}
	return false
}
