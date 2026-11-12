// Package logger menyediakan logger JSON terstruktur (slog) dengan nama service.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New membuat logger JSON ke stdout.
// Level yang didukung: debug, info, warn, error. Default info.
func New(service, level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h).With("service", service)
}
