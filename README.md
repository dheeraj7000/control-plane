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

No event bus, policy engine, budget tracking, or protocol adapters
exist yet — those are Milestones 3–5. No HTTP routes expose Execution/
Workflow yet either; that's Milestone 5's job. See
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
