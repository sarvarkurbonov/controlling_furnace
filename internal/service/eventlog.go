package service

import (
	"context"
	"controlling_furnace/internal/models"
	"errors"
	"strings"
	"time"

	"controlling_furnace/internal/repository"
)

type EventLogService struct {
	eventRepo repository.EventRepo
}

func NewEventLogService(eventRepo repository.EventRepo) *EventLogService {
	return &EventLogService{eventRepo: eventRepo}
}

var (
	errInvalidTimeRange = errors.New("invalid time range: From must be <= To")
)

// normalizeToUTC returns t in UTC, preserving zero time values.
func normalizeToUTC(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}

// normalizeEventType trims spaces and uppercases the event type filter.
func normalizeEventType(s string) string {
	return strings.TrimSpace(strings.ToUpper(s))
}

// normalizeAndValidateFilter prepares query parameters and validates the time range.
func normalizeAndValidateFilter(f LogFilter) (time.Time, time.Time, string, error) {
	from := normalizeToUTC(f.From)
	to := normalizeToUTC(f.To)

	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return time.Time{}, time.Time{}, "", errInvalidTimeRange
	}

	eventType := normalizeEventType(f.Type)
	return from, to, eventType, nil
}

func (s *EventLogService) List(ctx context.Context, f LogFilter) ([]models.FurnaceEvent, error) {
	from, to, typ, err := normalizeAndValidateFilter(f)
	if err != nil {
		return nil, err
	}
	return s.eventRepo.List(ctx, from, to, typ)
}
