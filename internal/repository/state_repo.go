package repository

import (
	"context"
	"controlling_furnace/internal/models"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type StateSQLite struct {
	db *sql.DB
}

func NewStateSQLite(db *sql.DB) *StateSQLite {
	return &StateSQLite{db: db}
}

// constants and helpers for clarity and reuse
const (
	furnaceStateRowID = 1

	insertOrUpdateStateSQL = `
		INSERT INTO furnace_state (id, mode, temp_c, target_c, remaining_s, errors, running, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			mode=excluded.mode,
			temp_c=excluded.temp_c,
			target_c=excluded.target_c,
			remaining_s=excluded.remaining_s,
			errors=excluded.errors,
			running=excluded.running,
			updated_at=excluded.updated_at
	`

	selectStateSQL = `
		SELECT id, mode, temp_c, target_c, remaining_s, errors, running, updated_at
		FROM furnace_state WHERE id=?
	`
)

// marshalErrorCodes converts the slice to a JSON string.
func marshalErrorCodes(codes []string) (string, error) {
	b, err := json.Marshal(codes)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// unmarshalErrorCodes parses a JSON string into a slice.
func unmarshalErrorCodes(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var codes []string
	if err := json.Unmarshal([]byte(s), &codes); err != nil {
		return nil, err
	}
	return codes, nil
}

// Save updates or inserts the furnace_state row (id always 1).
func (r *StateSQLite) Save(ctx context.Context, state models.FurnaceState) error {
	errorsJSONStr, err := marshalErrorCodes(state.ErrorCodes)
	if err != nil {
		return err
	}

	// ensure UpdatedAt is always persisted as UTC; set if zero
	tsUTC := state.UpdatedAt
	if tsUTC.IsZero() {
		tsUTC = time.Now().UTC()
	} else {
		tsUTC = tsUTC.UTC()
	}

	_, err = r.db.ExecContext(ctx, insertOrUpdateStateSQL,
		furnaceStateRowID,
		state.Mode,
		state.CurrentTempC,
		state.TargetTempC,
		state.RemainingSeconds,
		errorsJSONStr,
		state.IsRunning,
		tsUTC,
	)
	return err
}

// Load fetches the single furnace_state row (id=1).
func (r *StateSQLite) Load(ctx context.Context) (models.FurnaceState, error) {
	row := r.db.QueryRowContext(ctx, selectStateSQL, furnaceStateRowID)

	var s models.FurnaceState
	var errorsJSONStr string
	if err := row.Scan(
		&s.ID,
		&s.Mode,
		&s.CurrentTempC,
		&s.TargetTempC,
		&s.RemainingSeconds,
		&errorsJSONStr,
		&s.IsRunning,
		&s.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.FurnaceState{}, nil // no state yet
		}
		return models.FurnaceState{}, err
	}

	codes, err := unmarshalErrorCodes(errorsJSONStr)
	if err != nil {
		return models.FurnaceState{}, err
	}
	s.ErrorCodes = codes
	s.UpdatedAt = s.UpdatedAt.UTC()

	return s, nil
}
