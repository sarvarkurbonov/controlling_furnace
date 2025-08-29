// internal/service/monitoring_getstate_test.go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"controlling_furnace/internal/models"
)

// monitoringStateRepoStub is a local, uniquely named test stub that satisfies repository.StateRepo.
type monitoringStateRepoStub struct {
	loadResp   models.FurnaceState
	loadErr    error
	saveErr    error
	savedCalls []models.FurnaceState
}

func (s *monitoringStateRepoStub) Load(ctx context.Context) (models.FurnaceState, error) {
	return s.loadResp, s.loadErr
}

func (s *monitoringStateRepoStub) Save(ctx context.Context, state models.FurnaceState) error {
	s.savedCalls = append(s.savedCalls, state)
	return s.saveErr
}

func TestMonitoringService_GetState(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		repoResp   models.FurnaceState
		repoErr    error
		assertFunc func(t *testing.T, got models.FurnaceState, err error)
	}

	now := time.Now()

	cases := []testCase{
		{
			name:     "propagates repository error",
			repoErr:  errors.New("db down"),
			repoResp: models.FurnaceState{},
			assertFunc: func(t *testing.T, got models.FurnaceState, err error) {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				// Avoid struct comparison: inspect a sentinel field instead.
				if got.ID != 0 {
					t.Errorf("expected zero state ID, got %d", got.ID)
				}
			},
		},
		{
			name:     "returns baseline when no state (ID=0)",
			repoErr:  nil,
			repoResp: models.FurnaceState{ID: 0},
			assertFunc: func(t *testing.T, got models.FurnaceState, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ID != 1 {
					t.Errorf("baseline ID: want 1, got %d", got.ID)
				}
				if got.Mode != modeStandby {
					t.Errorf("baseline Mode: want %q, got %q", modeStandby, got.Mode)
				}
				if got.CurrentTempC != defaultAmbientTempC {
					t.Errorf("baseline CurrentTempC: want %v, got %v", defaultAmbientTempC, got.CurrentTempC)
				}
				if got.TargetTempC != 0 {
					t.Errorf("baseline TargetTempC: want 0, got %v", got.TargetTempC)
				}
				if got.RemainingSeconds != 0 {
					t.Errorf("baseline RemainingSeconds: want 0, got %d", got.RemainingSeconds)
				}
				if got.IsRunning {
					t.Errorf("baseline IsRunning: want false, got true")
				}
				if got.UpdatedAt.IsZero() {
					t.Fatalf("baseline UpdatedAt must be set, got zero")
				}
				if got.UpdatedAt.Location() != time.UTC {
					t.Errorf("baseline UpdatedAt must be UTC, got %v", got.UpdatedAt.Location())
				}
				assertWithin(t, got.UpdatedAt, time.Since(now)+200*time.Millisecond)
			},
		},
		{
			name:    "normalizes non-zero UpdatedAt to UTC for existing state",
			repoErr: nil,
			repoResp: models.FurnaceState{
				ID:           1,
				Mode:         "HEAT",
				CurrentTempC: 123.4,
				TargetTempC:  200,
				UpdatedAt:    time.Date(2025, 1, 2, 3, 4, 5, 0, time.FixedZone("X", -3*3600)), // UTC-3
			},
			assertFunc: func(t *testing.T, got models.FurnaceState, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ID != 1 {
					t.Fatalf("ID: want 1, got %d", got.ID)
				}
				if got.Mode != "HEAT" || got.CurrentTempC != 123.4 || got.TargetTempC != 200 {
					t.Errorf("unexpected state fields: %+v", got)
				}
				if got.UpdatedAt.Location() != time.UTC {
					t.Errorf("UpdatedAt must be UTC, got %v", got.UpdatedAt.Location())
				}
				wantUTC := time.Date(2025, 1, 2, 6, 4, 5, 0, time.UTC) // 03:04:05 -03:00 => 06:04:05 UTC
				if !got.UpdatedAt.Equal(wantUTC) {
					t.Errorf("UpdatedAt: want %v, got %v", wantUTC, got.UpdatedAt)
				}
			},
		},
		{
			name:    "preserves zero UpdatedAt for existing state",
			repoErr: nil,
			repoResp: models.FurnaceState{
				ID:           1,
				Mode:         "COOL",
				CurrentTempC: 20,
				TargetTempC:  10,
				UpdatedAt:    time.Time{},
			},
			assertFunc: func(t *testing.T, got models.FurnaceState, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !got.UpdatedAt.IsZero() {
					t.Errorf("UpdatedAt: want zero, got %v", got.UpdatedAt)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			repo := &monitoringStateRepoStub{
				loadResp: tc.repoResp,
				loadErr:  tc.repoErr,
			}

			svc := NewMonitoringService(repo)

			got, err := svc.GetState(ctx)
			tc.assertFunc(t, got, err)
		})
	}
}

func TestToUTC(t *testing.T) {
	t.Parallel()

	t.Run("zero time is preserved", func(t *testing.T) {
		t.Parallel()
		var z time.Time
		if got := toUTC(z); !got.IsZero() {
			t.Fatalf("expected zero time, got %v", got)
		}
	})

	t.Run("non-zero converted to UTC", func(t *testing.T) {
		t.Parallel()
		local := time.Date(2025, 2, 3, 10, 0, 0, 0, time.FixedZone("Z+2", 2*3600))
		got := toUTC(local)
		want := time.Date(2025, 2, 3, 8, 0, 0, 0, time.UTC)
		if got.Location() != time.UTC {
			t.Fatalf("expected UTC location, got %v", got.Location())
		}
		if !got.Equal(want) {
			t.Fatalf("want %v, got %v", want, got)
		}
	})
}

func TestMonitoringService_baselineState(t *testing.T) {
	t.Parallel()

	svc := NewMonitoringService(&monitoringStateRepoStub{})

	st := svc.baselineState()

	if st.ID != 1 {
		t.Errorf("ID: want 1, got %d", st.ID)
	}
	if st.Mode != modeStandby {
		t.Errorf("Mode: want %q, got %q", modeStandby, st.Mode)
	}
	if st.CurrentTempC != defaultAmbientTempC {
		t.Errorf("CurrentTempC: want %v, got %v", defaultAmbientTempC, st.CurrentTempC)
	}
	if st.TargetTempC != 0 {
		t.Errorf("TargetTempC: want 0, got %v", st.TargetTempC)
	}
	if st.RemainingSeconds != 0 {
		t.Errorf("RemainingSeconds: want 0, got %d", st.RemainingSeconds)
	}
	if st.IsRunning {
		t.Errorf("IsRunning: want false, got true")
	}
	if st.UpdatedAt.IsZero() {
		t.Fatalf("UpdatedAt must be set, got zero")
	}
	if st.UpdatedAt.Location() != time.UTC {
		t.Errorf("UpdatedAt: want UTC, got %v", st.UpdatedAt.Location())
	}
}

// assertWithin checks that got is within dur of now.
func assertWithin(t *testing.T, got time.Time, dur time.Duration) {
	t.Helper()
	if got.IsZero() {
		t.Fatalf("time is zero")
	}
	diff := time.Since(got)
	if diff < 0 {
		diff = -diff
	}
	if diff > dur {
		t.Fatalf("time %v not within %v of now; diff=%v", got, dur, diff)
	}
}
