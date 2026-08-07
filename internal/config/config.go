// Package config loads AI Agent Control Plane runtime configuration from
// environment variables. It is intentionally dependency-free (standard
// library only) so it can be imported by every apps/* binary without
// dragging in a config framework.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all environment-derived settings for the control plane
// server. Fields are grouped by the subsystem that consumes them.
type Config struct {
	// Env is the deployment environment: "development", "staging", "production".
	Env string
	// HTTPPort is the port the REST + WebSocket API listens on.
	HTTPPort int
	// LogLevel is one of: debug, info, warn, error.
	LogLevel string
	// ShutdownTimeout bounds how long graceful shutdown waits for
	// in-flight requests before forcing close.
	ShutdownTimeout time.Duration

	// DatabaseURL is the PostgreSQL connection string (postgres://...).
	DatabaseURL string
	// RedisURL is the Redis connection string (redis://...).
	RedisURL string
	// NATSURL is the NATS server URL (nats://...).
	NATSURL string

	// OTelExporter selects the OpenTelemetry trace exporter: "stdout",
	// "otlp", or "none". Defaults to "stdout" for local development.
	OTelExporter string
	// OTLPEndpoint is used when OTelExporter == "otlp".
	OTLPEndpoint string

	// ServiceName identifies this process in traces/metrics/logs.
	ServiceName string

	// MCPEndpoint is the MCP server's Streamable HTTP endpoint the
	// internal/adapters/mcp client calls for tool invocations.
	MCPEndpoint string
	// OpenAIBaseURL defaults to the real OpenAI API; override for an
	// OpenAI-compatible gateway or a test double.
	OpenAIBaseURL string
	// OpenAIAPIKey authenticates outbound OpenAI Chat Completions calls.
	// Empty is valid (model-call steps will simply fail against the
	// real API) — there's no local dev requirement to have a key.
	OpenAIAPIKey string

	// RateLimitPerMinute bounds requests per agent (or per source IP
	// for unauthenticated calls) via internal/gateway.RateLimiter.
	RateLimitPerMinute int

	// CORSAllowedOrigins is the set of origins the dashboard (or any
	// other browser-based client) may call this API from. Defaults to
	// "*" — permissive by design for local development, since there's
	// no deployment-specific origin to hardcode yet, but this is
	// exactly the kind of default a production deployment MUST
	// override (same posture as Milestone 5's default-allow policy
	// engine): see docs/architecture.md's open questions.
	CORSAllowedOrigins []string
}

// Load reads configuration from the process environment, applying
// sensible local-development defaults for anything unset. It returns an
// error if a set value is malformed (not if it's merely absent).
func Load() (*Config, error) {
	cfg := &Config{
		Env:           getEnv("APP_ENV", "development"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://control_plane:control_plane@localhost:5432/control_plane?sslmode=disable"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379/0"),
		NATSURL:       getEnv("NATS_URL", "nats://localhost:4222"),
		OTelExporter:  getEnv("OTEL_EXPORTER", "stdout"),
		OTLPEndpoint:  getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		ServiceName:   getEnv("SERVICE_NAME", "control-plane-server"),
		MCPEndpoint:   getEnv("MCP_ENDPOINT", "http://localhost:9000/mcp"),
		OpenAIBaseURL: getEnv("OPENAI_BASE_URL", ""),
		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
	}
	cfg.CORSAllowedOrigins = splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "*"))

	port, err := getEnvInt("HTTP_PORT", 8080)
	if err != nil {
		return nil, err
	}
	cfg.HTTPPort = port

	shutdownSeconds, err := getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", 15)
	if err != nil {
		return nil, err
	}
	cfg.ShutdownTimeout = time.Duration(shutdownSeconds) * time.Second

	rateLimit, err := getEnvInt("RATE_LIMIT_PER_MINUTE", 120)
	if err != nil {
		return nil, err
	}
	cfg.RateLimitPerMinute = rateLimit

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: invalid LOG_LEVEL %q (want debug|info|warn|error)", c.LogLevel)
	}
	switch c.OTelExporter {
	case "stdout", "otlp", "none":
	default:
		return fmt.Errorf("config: invalid OTEL_EXPORTER %q (want stdout|otlp|none)", c.OTelExporter)
	}
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("config: invalid HTTP_PORT %d", c.HTTPPort)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// splitCSV splits a comma-separated env value into a trimmed slice,
// dropping empty elements (so "a, ,b" and "a,b" behave the same).
func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid %s=%q: %w", key, v, err)
	}
	return n, nil
}
