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

**Milestone 4 — budget engine and native policy engine**

Budget had no milestone slot in the original plan despite being a named
Phase 1 deliverable, and Policy is specified to reference budget state
— so this milestone folds Budget in as Policy's prerequisite.

- `internal/budget`: `Ledger` tracks input/output tokens and cost
  (integer micro-USD, avoiding float drift) against a `Limit` at three
  scopes (execution/daily/monthly). Pure tracking only — whether an
  exceeded budget should deny anything is Policy's decision, not
  Budget's. `Repository`/`InMemoryRepository` follow the same
  clone-on-read/write discipline as Milestone 2's repositories.
- `internal/policy`: the spec's native evaluator behind an `Engine`
  interface (the named OPA/Cedar swap point). `NativeEngine` combines
  rules with **deny-overrides** semantics — any explicit deny wins
  immediately regardless of rule order, matching AWS IAM/Kubernetes
  admission-webhook conventions. Four concrete rules ship:
  `BudgetRule`, `ToolAllowlistRule`, `AllowedModelsRule`,
  `TimeWindowRule`, covering budget/agent/tool/model/time from the
  spec's named dimensions; workflow/execution/environment are already
  `Input` fields for a future rule, not yet backed by one.
- **A gap made explicit, not silently patched**: the spec names "Agent"
  as a core domain concept but its own repository layout never gives it
  a package. Budget/Policy both use a plain opaque owner-ID string
  rather than a fabricated `internal/agent` aggregate — which also
  means the spec's "Register an agent and its permissions" success
  criterion is **still not implemented**. See `docs/architecture.md`
  for the reasoning and what Milestone 5 needs to resolve.
- 97.8% / 100% unit test coverage; verified `internal/policy` imports
  only `internal/budget` and `internal/budget` imports no other
  internal package — the same decoupling discipline as Events/Timeline.

**Milestone 5 — Gateway and protocol adapters**

Every previously-isolated package gets composed for the first time, and
the server gets its first real HTTP surface.

- `internal/agent`: a new top-level package (the call made on Milestone
  4's open question) — identity, an allowed-tools list, and a bearer
  token whose plaintext is returned exactly once and only a salted hash
  is ever stored.
- `internal/adapters` (+ `mcp`, `openai`): two interfaces split by
  shape — `Adapter` for tool calls, `ModelAdapter` for model calls. The
  MCP client speaks real JSON-RPC 2.0 over MCP's Streamable HTTP
  transport for `tools/call` (simplified: no handshake/discovery/SSE,
  documented as a deliberate cut); the OpenAI client makes real HTTP
  calls to Chat Completions. Both tested against `httptest`, never a
  live API.
- `internal/gateway`: `Service` is the spec's "Execution Manager" —
  the first thing allowed to compose Workflow + Execution + Events +
  Policy + Budget + Adapters, which is exactly why those packages were
  kept ignorant of each other in Milestones 2-4. Its step-driver is
  deliberately synchronous/sequential (topological order, stop on first
  failure), not the concurrent Scheduler a production system needs —
  this proves the wiring, not the orchestration sophistication.
  `Mount()` wires the REST API (agents/workflows/executions +
  timeline/events), bearer-token auth, and a Redis-backed fixed-window
  rate limiter (fails open on a Redis error) onto a chi router.
  `GET .../executions/{id}/ws` streams live events — with no history
  for a late subscriber, a documented consequence of Milestone 3's Bus
  design.
- **Verified against a real running server**, not just unit tests:
  registered an agent, registered a workflow, started an execution over
  real HTTP, and watched it reach `completed` with the *exact* event
  chain from the spec's own worked example — against real
  Postgres/Redis/NATS.
- Known gaps, flagged rather than hidden: agent registration is
  unauthenticated (no control-plane-operator auth yet to gate it —
  `internal/auth` is still unbuilt), there's no stored/configurable
  Policy record yet (the wired `NativeEngine` has zero rules and
  default-allows), and tool-call idempotency / approval routing remain
  open from earlier milestones.

**Milestone 7 — persistence and production hardening**

Reordered ahead of Milestone 6 (Dashboard) — a dashboard needs real
data, and until this milestone every execution/workflow/agent vanished
on restart.

- `internal/storage`: Postgres-backed implementations of every domain
  `Repository` interface (plus `events.Store`), wired in as the default
  in `internal/app`. Migrations (embedded SQL, `golang-migrate`) run
  automatically on boot and are also exposed via `control-planectl
  migrate up`.
- New `Restore` constructors on `workflow`/`execution`/`agent` (the
  reconstitution gap flagged since Milestone 2) let a Postgres row
  become a domain object without going through each package's
  "fresh creation" constructor.
- Atomic per-execution event sequencing via a dedicated counter table
  (`INSERT ... ON CONFLICT DO UPDATE ... RETURNING`) — verified with a
  real 50-goroutine concurrency test asserting a gapless sequence, not
  just asserted correct.
- **Verified with an actual restart**: registered an agent/workflow/
  execution over real HTTP, killed the server, started a *new*
  process, and confirmed everything — including the agent's bearer
  token still authenticating — was exactly as left.
- Integration tests are real (gated behind `TEST_DATABASE_URL`, skip
  when unset), with a dedicated Postgres service in CI.
- Fixed a pre-existing latent bug found while here: `golangci-lint`
  v1.62.2 (pinned since Milestone 1) silently doesn't support the Go
  1.26 toolchain in `go.mod` — both the Makefile and CI now pin the
  version actually verified working throughout this project.
- Modest hardening: request bodies capped at 1 MiB.
- **Bus (live event fan-out) stays in-memory** — only durability moved
  to Postgres; a NATS-backed Bus remains deferred until something needs
  cross-instance delivery. Execution-write concurrency across multiple
  server instances (racing `UPDATE`s, no version/CAS check) is also
  still open — deliberately out of scope for "persistence exists and
  is correct for a single writer."

**Milestone 6 — dashboard**

Run after Milestone 7 (a dashboard needs real, persistent data to
point at). `apps/dashboard` is a Next.js/React/TypeScript app
rendering exactly what the REST/WebSocket API already exposes:
Overview, Agents, Workflows (+ step-graph detail via React Flow),
Executions (+ live detail: step graph with live per-step status,
timeline, raw events, budget usage), Settings.

- Two small, real backend changes were needed to make this possible:
  **CORS** (`internal/app` now mounts `go-chi/cors`, configured via
  `CORS_ALLOWED_ORIGINS`, default `*` for local dev) and a new
  **`GET .../executions/{id}/budget`** endpoint (nothing exposed a
  budget ledger's running usage over HTTP before now — `events.
  BudgetUpdated` only carries the per-charge delta).
- `events.Event` and `timeline.Entry` gained proper snake_case JSON
  tags — both had been HTTP response bodies since Milestone 5 with
  nobody noticing `encoding/json`'s no-tags fallback was shipping
  `PascalCase` field names, since nothing had decoded them back into a
  typed struct until the dashboard needed to.
- The WebSocket route (`GET .../executions/{id}/ws`) now also accepts
  its bearer token via a `?token=` query parameter — browsers can't
  set an `Authorization` header during a WebSocket handshake — via a
  new `AuthMiddlewareWS`, scoped to only that one route rather than
  relaxing the header-only rule everywhere.
- Component primitives (`Button`, `Card`, `Table`, `Dialog`, ...) are
  hand-rolled against shadcn/ui's design tokens rather than
  CLI-vendored, `@xyflow/react` renders workflow DAGs with a simple
  longest-path layout, and per-step execution status is derived
  client-side from the event stream rather than added to the API — see
  `apps/dashboard/README.md` and `docs/architecture.md` for the full
  reasoning.
- **Cut, deliberately**: Policies, Tool Registry, and Model Router
  pages don't exist — there's no admin API yet for any of those (see
  open question #7), and a page over state the backend can't persist
  would be a mockup, not a feature.
- **Verified against the real running stack**: registered an agent,
  workflow, and execution through the live Go server via curl,
  confirmed the dashboard serves every route against that same data,
  and confirmed the WebSocket's new query-token auth path with both a
  Go test and a standalone Python client against the live server.

See [`docs/architecture.md`](docs/architecture.md) for the full milestone
plan and the open design questions carried into them.

## Repository layout

```
apps/
  server/      REST + WebSocket API process (thin main, wiring lives in internal/app)
  dashboard/   Next.js dashboard — Milestone 6, see apps/dashboard/README.md
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
| `make build`            | Build `server` and `control-planectl` binaries |
| `make test`             | Run unit tests with the race detector (Postgres integration tests skip themselves) |
| `make test-integration` | Run `internal/storage`'s real Postgres integration tests (needs `make dev-up`) |
| `make migrate-up`       | Apply pending migrations by hand (the server also does this automatically on boot) |
| `make check`            | fmt + vet + test — run before committing       |
| `make lint`             | golangci-lint (auto-installs to `./bin`)       |
| `make dev-up`           | Start local infra only                         |
| `make docker-up`        | Build and run the full stack in containers (server + dashboard) |
| `make dashboard-dev`    | Run the dashboard's Next.js dev server         |
| `make dashboard-build`  | Lint, type-check, and production-build the dashboard |

## Requirements

- Go 1.25+ (developed against 1.26)
- Node.js 20+ (for `apps/dashboard`)
- Docker + Docker Compose v2
