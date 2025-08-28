package service

import "time"

type ModeParams struct {
	Mode        string  // "HEAT" | "COOL" | "STANDBY"
	TargetTempC float64 // only used when Mode == "HEAT"
	DurationSec int     // only used when Mode == "HEAT"
}

// LogFilter supports history filtering by time range and type (per test).
type LogFilter struct {
	From time.Time // inclusive; zero means no lower bound
	To   time.Time // inclusive; zero means no upper bound
	Type string    // "", "START", "STOP", "MODE_CHANGE", "ERROR", "TELEMETRY"
}
