package controlling_furnace

import "time"

//const (
//	ModeHeat    = "HEAT"
//	ModeCool    = "COOL"
//	ModeStandby = "STANDBY"
//)
//
//const (
//	EventStart      = "START"
//	EventStop       = "STOP"
//	EventModeChange = "MODE_CHANGE"
//	EventError      = "ERROR"
//	EventTelemetry  = "TELEMETRY"
//)

// FurnaceState is the current snapshot of the furnace.
type FurnaceState struct {
	ID               int       `json:"id"`
	Mode             string    `json:"mode"`                        // HEAT | COOL | STANDBY
	CurrentTempC     float64   `json:"current_temp_c"`              // °C
	TargetTempC      float64   `json:"target_temp_c,omitempty"`     // °C
	RemainingSeconds int       `json:"remaining_seconds,omitempty"` // seconds
	ErrorCodes       []string  `json:"error_codes,omitempty"`       // e.g. ["OVERHEAT", "SENSOR_FAULT"]
	IsRunning        bool      `json:"is_running"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// FurnaceEvent is a single log entry.
type FurnaceEvent struct {
	EventID     string    `json:"event_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	Type        string    `json:"type"`        // START | STOP | MODE_CHANGE | ERROR | TELEMETRY
	Description string    `json:"description"` // human-readable
	Metadata    any       `json:"metadata,omitempty"`
}
type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"` // don’t expose hash
}
