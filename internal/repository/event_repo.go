package repository

import (
	"context"
	"controlling_furnace/internal/models"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EventSQLite struct {
	db *sql.DB
}

func NewEventSQLite(db *sql.DB) *EventSQLite { return &EventSQLite{db: db} }

// Append inserts a new event. If EventID or OccurredAt are empty, they’re set.
func (r *EventSQLite) Append(ctx context.Context, e models.FurnaceEvent) error {
	if e.EventID == "" {
		e.EventID = uuid.NewString()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	} else {
		e.OccurredAt = e.OccurredAt.UTC()
	}

	// marshal metadata if present
	var metaPtr *string
	if e.Metadata != nil {
		if b, err := json.Marshal(e.Metadata); err == nil {
			s := string(b)
			metaPtr = &s
		}
	}

	// Insert with SQLite TIMESTAMP format "YYYY-MM-DD HH:MM:SS"
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO furnace_events (id, occurred_at, type, message, meta)
		VALUES (?, ?, ?, ?, ?)
	`,
		e.EventID,
		e.OccurredAt.Format("2006-01-02 15:04:05"), // ✅ SQLite TIMESTAMP format
		strings.ToUpper(strings.TrimSpace(e.Type)),
		e.Description,
		metaPtr,
	)

	return err
}

// List returns events filtered by [from, to] (inclusive) and/or type, ordered ASC.
func (r *EventSQLite) List(ctx context.Context, from, to time.Time, typ string) ([]models.FurnaceEvent, error) {
	var (
		conds []string
		args  []any
	)

	if !from.IsZero() {
		conds = append(conds, "occurred_at >= ?")
		args = append(args, from.UTC())
	}
	if !to.IsZero() {
		conds = append(conds, "occurred_at <= ?")
		args = append(args, to.UTC())
	}
	if typ = strings.ToUpper(strings.TrimSpace(typ)); typ != "" {
		conds = append(conds, "type = ?")
		args = append(args, typ)
	}

	q := `SELECT id, occurred_at, type, message, meta FROM furnace_events`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY occurred_at ASC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.FurnaceEvent, 0, 64)
	for rows.Next() {
		var ev models.FurnaceEvent
		var metaStr sql.NullString
		if err := rows.Scan(&ev.EventID, &ev.OccurredAt, &ev.Type, &ev.Description, &metaStr); err != nil {
			return nil, err
		}
		ev.OccurredAt = ev.OccurredAt.UTC()

		if metaStr.Valid && metaStr.String != "" {
			var v any
			if err := json.Unmarshal([]byte(metaStr.String), &v); err == nil {
				ev.Metadata = v
			} else {
				ev.Metadata = metaStr.String // keep raw if malformed
			}
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
