package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap's SugaredLogger.
type Logger struct {
	*zap.SugaredLogger
}

// defaultZapLevel defines the fallback log level when an unknown level string is provided.
const defaultZapLevel = zapcore.DebugLevel

// toZapLevel converts a textual level to zapcore.Level using known level constants.
func toZapLevel(levelStr string) zapcore.Level {
	switch levelStr {
	case InfoLevel:
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	default:
		return defaultZapLevel
	}
}

// newConsoleCore builds a zapcore.Core with a console encoder targeting stdout.
func newConsoleCore(level zapcore.Level) zapcore.Core {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = ""
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder

	encoder := zapcore.NewConsoleEncoder(cfg)
	ws := zapcore.Lock(os.Stdout) // thread-safe writer
	return zapcore.NewCore(encoder, zapcore.AddSync(ws), zap.NewAtomicLevelAt(level))
}

// newZapLogger constructs a sugared zap logger with the provided level string.
func newZapLogger(levelStr string) *Logger {
	core := newConsoleCore(toZapLevel(levelStr))
	return &Logger{
		SugaredLogger: zap.New(core).Sugar(),
	}
}
