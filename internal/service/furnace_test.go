package service

import (
	"context"
	"controlling_furnace/internal/models"
	"errors"
	"testing"
	"time"
)

type fakeStateRepo struct {
	loadResp   models.FurnaceState
	loadErr    error
	saveErr    error
	savedCalls []models.FurnaceState
}

func (f *fakeStateRepo) Load(ctx context.Context) (models.FurnaceState, error) {
	return f.loadResp, f.loadErr
}
func (f *fakeStateRepo) Save(ctx context.Context, s models.FurnaceState) error {
	f.savedCalls = append(f.savedCalls, s)
	return f.saveErr
}

type localEventRepo struct {
	appendErr error
	events    []models.FurnaceEvent
	listErr   error
}

func (f *localEventRepo) Append(ctx context.Context, e models.FurnaceEvent) error {
	f.events = append(f.events, e)
	return f.appendErr
}
func (f *localEventRepo) List(ctx context.Context, from time.Time, to time.Time, typ string) ([]models.FurnaceEvent, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []models.FurnaceEvent
	for _, e := range f.events {
		if !e.OccurredAt.Before(from) && !e.OccurredAt.After(to) {
			if typ == "" || e.Type == typ {
				out = append(out, e)
			}
		}
	}
	return out, nil
}
func assertWithinTimeWindow(t *testing.T, ts time.Time, start time.Time, end time.Time) {
	t.Helper()
	if ts.Before(start) || ts.After(end) {
		t.Fatalf("time %v not within window [%v, %v]", ts, start, end)
	}
}
func lastSavedState(t *testing.T, f *fakeStateRepo) models.FurnaceState {
	t.Helper()
	if len(f.savedCalls) == 0 {
		t.Fatalf("expected at least one Save call")
	}
	return f.savedCalls[len(f.savedCalls)-1]
}

// ... existing code ...
func TestFurnaceService_Start_LoadError(t *testing.T) {
	fs := &FurnaceService{
		stateRepo: &fakeStateRepo{loadErr: errors.New("db down")},
		eventRepo: &localEventRepo{},
	}
	err := fs.Start(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// ... existing code ...
func TestFurnaceService_Start_InitializesDefaultStateAndAppendsEvent(t *testing.T) {
	srepo := &fakeStateRepo{
		loadResp: models.FurnaceState{},
	}
	erepo := &localEventRepo{}
	fs := &FurnaceService{stateRepo: srepo, eventRepo: erepo}
	t0 := time.Now().UTC()
	err := fs.Start(context.Background())
	t1 := time.Now().UTC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := lastSavedState(t, srepo)
	if s.ID != 1 {
		t.Fatalf("expected ID=1, got %d", s.ID)
	}
	if !s.IsRunning {
		t.Fatalf("expected IsRunning=true")
	}
	if s.Mode != "STANDBY" {
		t.Fatalf("expected Mode=STANDBY, got %s", s.Mode)
	}
	if s.TargetTempC != 0 || s.RemainingSeconds != 0 {
		t.Fatalf("expected target/duration cleared, got target=%.1f duration=%d", s.TargetTempC, s.RemainingSeconds)
	}
	assertWithinTimeWindow(t, s.UpdatedAt, t0, t1)
	if len(erepo.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(erepo.events))
	}
	ev := erepo.events[0]
	if ev.Type != "START" {
		t.Fatalf("expected START event, got %s", ev.Type)
	}
	if ev.EventID == "" {
		t.Fatalf("expected non-empty EventID")
	}
	assertWithinTimeWindow(t, ev.OccurredAt, t0, t1)
}

// ... existing code ...
func TestFurnaceService_Start_ExistingStateSetsRunningAndAppendsEvent(t *testing.T) {
	srepo := &fakeStateRepo{
		loadResp: models.FurnaceState{
			ID:               1,
			Mode:             "COOL",
			CurrentTempC:     30,
			TargetTempC:      15,
			RemainingSeconds: 42,
			IsRunning:        false,
			UpdatedAt:        time.Unix(0, 0),
		},
	}
	erepo := &localEventRepo{}
	fs := &FurnaceService{stateRepo: srepo, eventRepo: erepo}
	t0 := time.Now().UTC()
	err := fs.Start(context.Background())
	t1 := time.Now().UTC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := lastSavedState(t, srepo)
	if s.ID != 1 {
		t.Fatalf("expected ID=1, got %d", s.ID)
	}
	if !s.IsRunning {
		t.Fatalf("expected IsRunning=true")
	}
	assertWithinTimeWindow(t, s.UpdatedAt, t0, t1)
	if len(erepo.events) != 1 || erepo.events[0].Type != "START" {
		t.Fatalf("expected START event, got %#v", erepo.events)
	}
}

// ... existing code ...
func TestFurnaceService_Stop_BaselineWhenNoStateAndAppendsEvent(t *testing.T) {
	srepo := &fakeStateRepo{
		loadResp: models.FurnaceState{},
	}
	erepo := &localEventRepo{}
	fs := &FurnaceService{stateRepo: srepo, eventRepo: erepo}
	t0 := time.Now().UTC()
	err := fs.Stop(context.Background())
	t1 := time.Now().UTC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := lastSavedState(t, srepo)
	if s.ID != 1 {
		t.Fatalf("expected ID=1 when stopping without prior state")
	}
	if s.IsRunning {
		t.Fatalf("expected IsRunning=false")
	}
	if s.Mode != "STANDBY" {
		t.Fatalf("expected Mode=STANDBY, got %s", s.Mode)
	}
	if s.TargetTempC != 0 || s.RemainingSeconds != 0 {
		t.Fatalf("expected target/duration cleared")
	}
	assertWithinTimeWindow(t, s.UpdatedAt, t0, t1)
	if len(erepo.events) != 1 || erepo.events[0].Type != "STOP" {
		t.Fatalf("expected STOP event, got %#v", erepo.events)
	}
}

// ... existing code ...
