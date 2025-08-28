package logger

import (
	"sync"
)

// Log levels used across the application.
const (
	DebugLevel = "debug"
	InfoLevel  = "info"
	WarnLevel  = "warn"
	ErrorLevel = "error"
)

var (
	// globalLogger holds the singleton logger instance.
	globalLogger *Logger
	once         sync.Once
)

// Get returns a singleton logger configured with the provided level.
// The first call initializes the logger; subsequent calls ignore the level
// and return the already initialized instance.
func Get(level string) *Logger {
	once.Do(func() {
		globalLogger = newZapLogger(level)
	})
	return globalLogger
}
