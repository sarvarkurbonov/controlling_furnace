package models

import "time"

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
