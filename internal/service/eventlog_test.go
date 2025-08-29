package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"controlling_furnace/internal/models"
)

// fakeEventRepo is a minimal stub that satisfies the repository.EventRepo interface.
type fakeEventRepo struct {
	// captured inputs
	gotCtx  context.Context
	gotFrom time.Time
	gotTo   time.Time
	gotType string

	// configured outputs
	events []models.FurnaceEvent
	err    error

	calls int
}

func (f *fakeEventRepo) List(ctx context.Context, from, to time.Time, typ string) ([]models.FurnaceEvent, error) {
	f.calls++
	f.gotCtx = ctx
	f.gotFrom = from
	f.gotTo = to
	f.gotType = typ
	return f.events, f.err
}

// helpers
func (f *fakeEventRepo) Append(ctx context.Context, e models.FurnaceEvent) error {
	return nil
}

func fixedZone(name string, offsetSec int) *time.Location {
	return time.FixedZone(name, offsetSec)
}

func mustTimeIn(loc *time.Location, y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, loc)
}

// normalizeToUTC

func Test_normalizeToUTC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Time
		want func(time.Time) bool
	}{
		{
			name: "zero time remains zero",
			in:   time.Time{},
			want: func(out time.Time) bool { return out.IsZero() },
		},
		{
			name: "non-UTC converted to UTC preserving instant",
			in:   mustTimeIn(fixedZone("UTC+3", 3*3600), 2025, time.August, 1, 12, 34, 56),
			want: func(out time.Time) bool {
				exp := time.Date(2025, time.August, 1, 9, 34, 56, 0, time.UTC) // 12:34:56+03 == 09:34:56Z
				return out.Location() == time.UTC && out.Equal(exp)
			},
		},
		{
			name: "already UTC stays UTC and same instant",
			in:   time.Date(2025, time.August, 2, 0, 0, 0, 0, time.UTC),
			want: func(out time.Time) bool {
				exp := time.Date(2025, time.August, 2, 0, 0, 0, 0, time.UTC)
				return out.Location() == time.UTC && out.Equal(exp)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeToUTC(tc.in)
			if !tc.want(got) {
				t.Fatalf("unexpected normalizeToUTC result: %v (loc=%v)", got, got.Location())
			}
		})
	}
}

// normalizeEventType

func Test_normalizeEventType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{name: "empty stays empty", in: "", exp: ""},
		{name: "trim spaces", in: "  START ", exp: "START"},
		{name: "uppercase", in: "error", exp: "ERROR"},
		{name: "spaces preserved except ends", in: " mode_change ", exp: "MODE_CHANGE"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeEventType(c.in)
			if got != c.exp {
				t.Fatalf("normalizeEventType(%q) = %q; want %q", c.in, got, c.exp)
			}
		})
	}
}

// normalizeAndValidateFilter

func Test_normalizeAndValidateFilter(t *testing.T) {
	t.Parallel()

	fromLocal := mustTimeIn(fixedZone("UTC+2", 2*3600), 2025, time.September, 10, 10, 0, 0)
	toUTC := time.Date(2025, time.September, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		in       LogFilter
		wantFrom time.Time
		wantTo   time.Time
		wantType string
		wantErr  error
	}{
		{
			name:     "all zero/empty ok",
			in:       LogFilter{},
			wantFrom: time.Time{},
			wantTo:   time.Time{},
			wantType: "",
			wantErr:  nil,
		},
		{
			name: "from after to -> error",
			in: LogFilter{
				From: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
				To:   time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC),
				Type: "start",
			},
			wantErr: errInvalidTimeRange,
		},
		{
			name: "normalize tz and type",
			in: LogFilter{
				From: fromLocal,
				To:   toUTC,
				Type: " start ",
			},
			wantFrom: time.Date(2025, time.September, 10, 8, 0, 0, 0, time.UTC), // 10:00 +02 -> 08:00Z
			wantTo:   toUTC,
			wantType: "START",
			wantErr:  nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotFrom, gotTo, gotType, err := normalizeAndValidateFilter(tc.in)

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected err %v; got %v", tc.wantErr, err)
			}
			// Only assert non-zero expectations for times
			if !tc.wantFrom.IsZero() && !gotFrom.Equal(tc.wantFrom) {
				t.Fatalf("from: got %v; want %v", gotFrom, tc.wantFrom)
			}
			if !tc.wantTo.IsZero() && !gotTo.Equal(tc.wantTo) {
				t.Fatalf("to: got %v; want %v", gotTo, tc.wantTo)
			}
			if tc.wantType != "" && gotType != tc.wantType {
				t.Fatalf("type: got %q; want %q", gotType, tc.wantType)
			}
		})
	}
}

// EventLogService.List

func TestEventLogService_List_DelegatesNormalizedParams(t *testing.T) {
	t.Parallel()

	frepo := &fakeEventRepo{
		events: []models.FurnaceEvent{
			{EventID: "1"},
		},
	}
	svc := NewEventLogService(frepo)

	fromLocal := mustTimeIn(fixedZone("UTC+5", 5*3600), 2025, time.October, 1, 10, 0, 0)
	toLocal := mustTimeIn(fixedZone("UTC-2", -2*3600), 2025, time.October, 1, 12, 30, 0)
	ctx := context.Background()

	out, err := svc.List(ctx, LogFilter{
		From: fromLocal,
		To:   toLocal,
		Type: "  error ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].EventID != "1" {
		t.Fatalf("unexpected events: %+v", out)
	}
	if frepo.calls != 1 {
		t.Fatalf("repo List should be called once, got %d", frepo.calls)
	}

	// Check normalized values passed to repo
	wantFrom := time.Date(2025, time.October, 1, 5, 0, 0, 0, time.UTC) // 10:00 +05 -> 05:00Z
	wantTo := time.Date(2025, time.October, 1, 14, 30, 0, 0, time.UTC) // 12:30 -02 -> 14:30Z

	if !frepo.gotFrom.Equal(wantFrom) {
		t.Fatalf("repo gotFrom=%v; want %v", frepo.gotFrom, wantFrom)
	}
	if !frepo.gotTo.Equal(wantTo) {
		t.Fatalf("repo gotTo=%v; want %v", frepo.gotTo, wantTo)
	}
	if frepo.gotType != "ERROR" {
		t.Fatalf("repo gotType=%q; want %q", frepo.gotType, "ERROR")
	}
}

func TestEventLogService_List_ValidationError(t *testing.T) {
	t.Parallel()

	frepo := &fakeEventRepo{}
	svc := NewEventLogService(frepo)

	_, err := svc.List(context.Background(), LogFilter{
		From: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, errInvalidTimeRange) {
		t.Fatalf("expected errInvalidTimeRange; got %v", err)
	}
	if frepo.calls != 0 {
		t.Fatalf("repo should not be called on validation error, calls=%d", frepo.calls)
	}
}

func TestEventLogService_List_RepoErrorPropagation(t *testing.T) {
	t.Parallel()

	frepo := &fakeEventRepo{err: errors.New("db down")}
	svc := NewEventLogService(frepo)

	_, err := svc.List(context.Background(), LogFilter{})
	if !errors.Is(err, frepo.err) {
		t.Fatalf("expected repo error to propagate; got %v", err)
	}
	if frepo.calls != 1 {
		t.Fatalf("repo should be called once, calls=%d", frepo.calls)
	}
}

func TestEventLogService_List_ZeroBoundsPassedAsZero(t *testing.T) {
	t.Parallel()

	frepo := &fakeEventRepo{}
	svc := NewEventLogService(frepo)

	_, err := svc.List(context.Background(), LogFilter{
		From: time.Time{},
		To:   time.Time{},
		Type: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !frepo.gotFrom.IsZero() || !frepo.gotTo.IsZero() || frepo.gotType != "" {
		t.Fatalf("expected zero bounds and empty type; got from=%v to=%v type=%q", frepo.gotFrom, frepo.gotTo, frepo.gotType)
	}
}
