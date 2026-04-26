package log

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// NewJson configures and returns a new slog.Logger based on the provided level string.
// It parses the level string (case-insensitive) and creates a JSON logger
// writing to standard error.
func NewJson(levelStr string) *slog.Logger {
	var logLevel slog.Level

	switch strings.ToLower(levelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		// Default is Info
		logLevel = slog.LevelInfo
		fmt.Printf("Warning: Invalid log level '%s' provided, defaulting to 'info'.\n", levelStr)
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	return slog.New(jsonHandler)
}

func NewText(levelStr string) *slog.Logger {
	var logLevel slog.Level

	switch strings.ToLower(levelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		// Default is Info
		logLevel = slog.LevelInfo
		fmt.Printf("Warning: Invalid log level '%s' provided, defaulting to 'info'.\n", levelStr)
	}

	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel, AddSource: true})
	return slog.New(textHandler)
}

func Fatal(l *slog.Logger, err error, msg string, args ...any) {
	newArgs := append([]any{"error", err}, args)
	l.Error(msg, newArgs...)
	os.Exit(1)
}
