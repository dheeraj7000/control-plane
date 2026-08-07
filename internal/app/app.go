// Package app is the composition root for the control plane server. It
// owns construction order and lifetime of every shared dependency
// (config, logger, tracer, database pool, cache client, message bus)
// and hands them to subsystems via plain constructor injection — no DI
// framework/magic, per the project's "interface-first, no layered MVC"
// design principles.
//
// Domain packages (internal/execution, internal/workflow, ...) know
// nothing about this package. This package knows about all of them.
// That dependency direction is what keeps the domain testable in
// isolation and keeps this file the single place wiring changes when a
// new subsystem is added in a later milestone.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	"github.com/dheeraj7000/control-plane/internal/adapters/mcp"
	"github.com/dheeraj7000/control-plane/internal/adapters/openai"
	"github.com/dheeraj7000/control-plane/internal/agent"
	"github.com/dheeraj7000/control-plane/internal/budget"
	"github.com/dheeraj7000/control-plane/internal/config"
	"github.com/dheeraj7000/control-plane/internal/events"
	"github.com/dheeraj7000/control-plane/internal/execution"
	"github.com/dheeraj7000/control-plane/internal/gateway"
	"github.com/dheeraj7000/control-plane/internal/policy"
	"github.com/dheeraj7000/control-plane/internal/telemetry"
	"github.com/dheeraj7000/control-plane/internal/workflow"
	pkgserver "github.com/dheeraj7000/control-plane/pkg/server"
)

// App holds every dependency shared across the control plane server. It
// is the concrete type that gets built once in main() and threaded
// through to route handlers as subsequent milestones add them.
type App struct {
	Config    *config.Config
	Logger    *slog.Logger
	Telemetry *telemetry.Provider

	DB    *pgxpool.Pool // lazy-connecting; real usage starts in Milestone 7
	Redis *redis.Client // lazy-connecting
	NATS  *nats.Conn    // best-effort eager connect, see connectNATS

	// Domain repositories. All in-memory today (see each package's
	// InMemoryRepository) — Milestone 7 swaps DB/Redis-backed
	// implementations in here without touching callers.
	Workflows  workflow.Repository
	Executions execution.Repository
	Agents     agent.Repository
	Budgets    budget.Repository
	Events     *events.Recorder

	// Gateway is the orchestrator (Milestone 5's "Execution Manager")
	// composing everything above with Policy and the protocol adapters.
	Gateway *gateway.Service

	Router     *chi.Mux
	httpServer *http.Server
}

// New wires every dependency in the order later dependencies require:
// config -> logging -> telemetry -> infra clients -> router -> server.
// It deliberately does NOT hard-fail if downstream infra (Postgres,
// Redis, NATS) is unreachable at boot — Postgres/Redis clients in this
// stack connect lazily, and NATS degrades to nil with a logged warning.
// This lets `go run ./apps/server` and CI both succeed without Docker
// Compose running; the /readyz endpoint is what tells an operator
// whether infra is actually reachable.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	logger := telemetry.NewLogger(cfg.LogLevel, cfg.ServiceName)

	tp, err := telemetry.NewProvider(ctx, cfg.OTelExporter, cfg.OTLPEndpoint, cfg.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("app: init telemetry: %w", err)
	}

	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("app: init postgres pool: %w", err)
	}

	redisClient := redis.NewClient(mustParseRedisURL(cfg.RedisURL, logger))

	natsConn := connectNATS(cfg.NATSURL, logger)

	a := &App{
		Config:     cfg,
		Logger:     logger,
		Telemetry:  tp,
		DB:         dbPool,
		Redis:      redisClient,
		NATS:       natsConn,
		Workflows:  workflow.NewInMemoryRepository(),
		Executions: execution.NewInMemoryRepository(),
		Agents:     agent.NewInMemoryRepository(),
		Budgets:    budget.NewInMemoryRepository(),
		Events:     events.NewRecorder(events.NewInMemoryStore(), events.NewInMemoryBus()),
	}

	// No stored/toggleable policy rules exist yet (that's Milestone
	// 5+'s dashboard-managed Policy records, layered on the Rule/Engine
	// primitives Milestone 4 built) — default-allow with zero rules is
	// the only honest choice until there's a way to configure real
	// ones. Flagged here rather than silently shipping a permissive
	// posture: a production deployment MUST configure real rules
	// before this default matters.
	policyEngine, err := policy.NewNativeEngine(policy.EffectAllow)
	if err != nil {
		return nil, fmt.Errorf("app: init policy engine: %w", err)
	}

	gatewaySvc, err := gateway.NewService(gateway.ServiceConfig{
		Workflows:    a.Workflows,
		Executions:   a.Executions,
		Agents:       a.Agents,
		Budgets:      a.Budgets,
		Events:       a.Events,
		PolicyEngine: policyEngine,
		ToolAdapter:  mcp.New(cfg.MCPEndpoint, nil),
		ModelAdapter: openai.New(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey, nil),
		Logger:       logger,
		Environment:  cfg.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("app: init gateway service: %w", err)
	}
	a.Gateway = gatewaySvc

	a.Router = a.newRouter()
	a.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           a.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return a, nil
}

// newRouter builds the chi mux: health/readiness (owned directly by
// the composition root) plus every internal/gateway route, mounted
// here rather than gateway calling back into app — the composition
// root is the only place that should know about route wiring.
func (a *App) newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// NOTE: chi's middleware.RealIP is deliberately not used here — it
	// trusts X-Forwarded-For/X-Real-IP unconditionally, which is
	// spoofable unless you know your edge proxy strips/sets those
	// headers. Trusted-proxy-aware IP extraction needs to be
	// configured against the actual deployment topology, not
	// hardcoded into the composition root.
	r.Use(slogRequestLogger(a.Logger))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", a.handleLiveness)
	r.Get("/readyz", a.handleReadiness)

	limiter := gateway.NewRateLimiter(a.Redis, a.Config.RateLimitPerMinute, time.Minute)
	gateway.Mount(r, a.Gateway, a.Agents, limiter)

	return r
}

// Run blocks, serving HTTP until the process is signalled to stop, then
// drains in-flight requests per Config.ShutdownTimeout.
func (a *App) Run(ctx context.Context) error {
	return pkgserver.RunGraceful(ctx, a.httpServer, a.Logger, a.Config.ShutdownTimeout)
}

// Close releases every resource acquired in New. Call after Run returns.
func (a *App) Close(ctx context.Context) error {
	var errs []error

	a.DB.Close()

	if err := a.Redis.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close redis: %w", err))
	}

	if a.NATS != nil {
		a.NATS.Close()
	}

	if err := a.Telemetry.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("close telemetry: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("app: close: %v", errs)
	}
	return nil
}

func mustParseRedisURL(rawURL string, logger *slog.Logger) *redis.Options {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		// A malformed REDIS_URL is a config bug, but we still want the
		// rest of the server (and its /readyz diagnostics) to come up
		// so the operator can see *why* Redis is unreachable rather
		// than the process refusing to start at all.
		logger.Warn("invalid REDIS_URL, redis client will fail health checks",
			slog.String("error", err.Error()))
		return &redis.Options{Addr: "invalid:0"}
	}
	return opts
}

// connectNATS attempts a bounded, non-fatal connection to NATS. NATS
// (unlike the pgx/redis clients above) connects eagerly by default, so
// without a short timeout an unreachable broker would hang server
// startup. On failure we log and return nil; readiness checks report
// NATS as unavailable rather than crash-looping the whole process.
func connectNATS(url string, logger *slog.Logger) *nats.Conn {
	conn, err := nats.Connect(url,
		nats.Timeout(2*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		logger.Warn("nats unreachable at startup, continuing without it",
			slog.String("url", url), slog.String("error", err.Error()))
		return nil
	}
	return conn
}

func slogRequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}
