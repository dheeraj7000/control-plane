# AI Agent Control Plane

An open-source control plane for managing, securing, observing, and
orchestrating AI agents across MCP and non-MCP providers. The core
abstraction is the **Execution** — a running instance of a **Workflow**
template — not an HTTP request, not an agent. See
[`docs/architecture.md`](docs/architecture.md) for the full design and
the package-boundary decisions made while scaffolding this milestone.

## Status: Milestone 1 — repository scaffold, architecture, and DI

This milestone proves out the process skeleton, nothing more:

- Domain-driven package layout with every subsystem's responsibility
  documented in its `doc.go` (see `internal/*/doc.go`).
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

No Execution/Workflow domain logic, event bus, policy engine, or
adapters exist yet — those are Milestones 2–5. See
[`docs/architecture.md`](docs/architecture.md) for the full milestone
plan and the open design questions carried into them.

## Repository layout

```
apps/
  server/      REST + WebSocket API process (thin main, wiring lives in internal/app)
  dashboard/   Next.js dashboard (placeholder — Milestone 6)
  cli/         operator CLI (placeholder)
internal/      domain packages — see internal/*/doc.go for each one's responsibility
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
