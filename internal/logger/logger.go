package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is a structured logger wrapper around slog
type Logger struct {
	*slog.Logger
}

// New creates a new structured logger with the specified log level
func New(level string) *Logger {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{
		Logger: slog.New(handler),
	}
}

// WithFields creates a child logger with additional fields
func (l *Logger) WithFields(fields ...any) *Logger {
	return &Logger{
		Logger: l.With(fields...),
	}
}
