# AI Agent Control Plane

An open-source control plane for managing, securing, observing, and
orchestrating AI agents across MCP and non-MCP providers. The core
abstraction is the **Execution** — a running instance of a **Workflow**
template — not an HTTP request, not an agent. See
[`docs/architecture.md`](docs/architecture.md) for the full design and
the package-boundary decisions made along the way.

## Status

**Milestone 1 — repository scaffold, architecture, and DI**

- Domain-driven package layout with every subsystem's responsibility
  documented in a package comment.
- A single composition root (`internal/app`) wiring config, structured
  logging, OpenTelemetry tracing, and Postgres/Redis/NATS clients via
  plain constructor injection — no DI framework.
- `apps/server` boots, serves `/healthz` and `/readyz`, and shuts down
  gracefully on SIGINT/SIGTERM.
- `apps/cli` is a placeholder operator CLI.
- Docker Compose brings up Postgres, Redis, and NATS (JetStream-enabled)
  for local development.
- CI (`.github/workflows/ci.yml`) runs build, vet, tests, gofmt check,
  golangci-lint, and validates the Compose file.

**Milestone 2 — core domain model and state machine**

- `internal/workflow`: the immutable `Workflow`/`Step` aggregate.
  Construction fully validates the step graph (unique IDs, resolvable
  dependencies, cycle detection via Kahn's algorithm) and caches a
  topological order — a `Workflow` value cannot exist in an invalid
  state.
- `internal/execution`: the `Execution` aggregate — lifecycle `State`
  machine (Created/Queued/Running/Waiting/Paused/Retrying/Completed/
  Failed/Cancelled, every transition validated) plus per-step
  `StepRun` progress tracking. See `docs/architecture.md` for the
  corrected state-transition diagram (the spec's own diagram renders
  as a straight line, which isn't a real graph).
- Both packages ship a `Repository` interface and an
  `InMemoryRepository` — the real implementation used until Milestone
  7 swaps in Postgres, not just a test double.
- 90%+ unit test coverage on both packages.

**Milestone 3 — event bus and timeline engine**

- `internal/events`: the `Event` envelope (18 event types from the
  spec's named chain) plus two interfaces — `Store` (durable,
  replayable log; source of truth) and `Bus` (best-effort live pub/sub
  fan-out) — matching the Milestone 1 decision that NATS is fan-out and
  Postgres is durable. `InMemoryStore`/`InMemoryBus` are the real
  implementations used until later milestones swap in NATS/Postgres.
  `Recorder` ties the two together for producers.
- `internal/timeline`: a pure projection from an ordered event stream
  to human-readable `Entry` values (e.g. `Budget +3500 Tokens`,
  `Policy Filesystem Write Denied`, matching the spec's example). No
  dependency on `internal/execution` or `internal/workflow` — labels
  come from the event's own payload.
- This *is* "replay the execution timeline" in its safe, read-only
  sense: `timeline.Build` is deterministic over `Store.List`'s output.
  Re-executing side-effecting steps is a different, still out-of-scope
  capability — see `docs/architecture.md`.
- 97%+ / 98%+ unit test coverage on the two new packages, race-detector
  clean (the in-memory Bus/Store are the first genuinely concurrent
  code in this repo).

No policy engine, budget tracking, or protocol adapters exist yet —
those are Milestones 4–5. No HTTP/WebSocket routes were added either
(the spec's `/events` and `/timeline` streaming endpoints are Gateway's
job, Milestone 5) and nothing yet wires `internal/execution`'s state
transitions to actually emit events — that wiring belongs to the
Execution Manager, which doesn't exist until Milestone 5. See
[`docs/architecture.md`](docs/architecture.md) for the full milestone
plan and the open design questions carried into them.

## Repository layout

```
apps/
  server/      REST + WebSocket API process (thin main, wiring lives in internal/app)
  dashboard/   Next.js dashboard (placeholder — Milestone 6)
  cli/         operator CLI (placeholder)
internal/      domain packages — see each package's doc comment for its responsibility
pkg/           dependency-free code safe to reuse outside this module
api/           OpenAPI spec
docs/          architecture notes
deployments/   Dockerfiles / future k8s manifests
scripts/       dev tooling
examples/      example workflow definitions (Milestone 2+)
sdk/           generated client SDKs (later)
```

## Local development

```bash
cp .env.example .env        # or: ./scripts/bootstrap.sh
make dev-up                 # start Postgres, Redis, NATS
make run                    # go run ./apps/server
curl localhost:8080/healthz
curl localhost:8080/readyz
```

Or run everything, server included, in containers:

```bash
make docker-up
```

## Common tasks

| Command          | Description                                  |
|------------------|-----------------------------------------------|
| `make build`     | Build `server` and `control-planectl` binaries |
| `make test`      | Run unit tests with the race detector          |
| `make check`     | fmt + vet + test — run before committing       |
| `make lint`      | golangci-lint (auto-installs to `./bin`)       |
| `make dev-up`    | Start local infra only                         |
| `make docker-up` | Build and run the full stack in containers     |

## Requirements

- Go 1.25+ (developed against 1.26)
- Docker + Docker Compose v2
