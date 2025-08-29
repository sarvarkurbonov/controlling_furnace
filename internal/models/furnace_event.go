package models

import "time"

// FurnaceEvent is a single log entry.
type FurnaceEvent struct {
	EventID     string    `json:"event_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	Type        string    `json:"type"`        // START | STOP | MODE_CHANGE | ERROR | TELEMETRY
	Description string    `json:"description"` // human-readable
	Metadata    any       `json:"metadata,omitempty"`
}
