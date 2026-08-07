// Package telemetry wires up the control plane's cross-cutting
// observability: structured logging and OpenTelemetry tracing/metrics.
//
// Design principle #4 ("Observable by Default") means every subsystem
// gets a logger and tracer from here rather than constructing its own —
// this is the only place log format and exporter configuration live.
package telemetry

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger builds a JSON structured logger per the "Structured logging
// (JSON)" non-functional requirement. levelName is one of
// debug|info|warn|error; anything else falls back to info.
func NewLogger(levelName, serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(levelName),
	})
	return slog.New(handler).With(
		slog.String("service", serviceName),
	)
}

func parseLevel(name string) slog.Level {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
