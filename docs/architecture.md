# Architecture

## High-level shape

```mermaid
flowchart TB
    subgraph Client
        Dashboard[Dashboard]
    end

    Dashboard <--> API[REST + WebSocket API]

    subgraph ControlPlane[AI Agent Control Plane]
        API --> Gateway
        Gateway --> WorkflowEngine[Workflow Engine]
        Gateway --> ExecutionManager[Execution Manager]
        ExecutionManager --> PolicyEngine[Policy Engine]
        ExecutionManager --> BudgetEngine[Budget Engine]
        ExecutionManager --> Scheduler
        ExecutionManager --> EventBus
        EventBus --> TimelineEngine[Timeline Engine]
        EventBus --> Telemetry
        EventBus --> Audit
        Gateway --> ToolRegistry[Tool Registry]
        Gateway --> ModelRouter[Model Router]
        ModelRouter --> AdapterLayer[Adapter Layer]
        ToolRegistry --> AdapterLayer
    end

    AdapterLayer --> MCP
    AdapterLayer --> OpenAI
    AdapterLayer --> Anthropic
    AdapterLayer --> Ollama

    MCP --> ExternalTools[External Tools]
    OpenAI --> ExternalTools
    Anthropic --> ExternalTools
    Ollama --> ExternalTools
```

This mirrors the spec's box diagram directly; nothing here is a design
change, just a precise rendering so package boundaries can be checked
against it.

## Package-boundary decisions made during scaffolding

The spec left a few boundaries implicit. These are the calls made to
unblock Milestone 1 — flag any of them if they don't match intent, they're
cheap to change now and expensive after Milestone 5:

- **Gateway vs. Adapter Layer**: `internal/gateway` is the ingress
  boundary — inbound protocol termination, authn/authz, rate limiting —
  and calls into `internal/adapters` for provider-specific translation.
  `internal/adapters` never terminates a request itself.
- **Event store vs. message bus**: NATS (JetStream) is fan-out/delivery
  only (pub/sub to WebSocket subscribers, cross-instance coordination).
  PostgreSQL is the source of truth for the event log; anything durable
  (audit, replay) reads from Postgres, never from NATS.
- **Storage package split**: repository *interfaces* live next to the
  domain package that owns the aggregate (e.g. `internal/execution`
  defines `ExecutionRepository`). `internal/storage` holds only the
  Postgres implementations, added in Milestone 7. Earlier milestones
  develop against in-memory implementations of the same interfaces.
- **Dependency injection**: no DI framework. `internal/app` is a single
  composition root using constructor injection — see `internal/app/app.go`.
  Domain packages depend on nothing in `internal/app`; `internal/app`
  depends on all of them. This keeps domain logic unit-testable without
  a DI container and keeps "what gets wired where" in one file.
- **Config**: environment-variable driven (`internal/config`), no
  framework, since the non-functional requirement asks for env-var
  config and nothing more sophisticated is needed yet.

## Deferred to later milestones (see each package's doc comment)

Gateway, Policy evaluator, Budget tracking, Event envelope/bus, Timeline
projection, Model Router, and all provider adapters are still empty
packages with a documented responsibility and target milestone — see
the package comment at the top of each `internal/*` package's main file.
Execution and Workflow (below) are no longer in that category.

## Milestone 2 — core domain model and state machine

`internal/workflow` and `internal/execution` are implemented. Key
decisions made to unblock this milestone, resolving two of the open
questions below:

- **Workflow definition format** (was open question #4): canonical
  in-memory representation is the `Workflow`/`Step` Go structs, with
  `json` tags for API request/response bodies. YAML is left as a thin
  conversion layer to add later (unmarshal YAML → JSON → the same
  struct) rather than a distinct format designed around now — this
  keeps the REST API (Milestone 5) trivial and doesn't invent a DSL
  before there's evidence one is needed.
- **Make illegal states unrepresentable**: `workflow.New` fully
  validates the step graph (unique IDs, resolvable dependencies, no
  cycles — detected via Kahn's algorithm) and caches a topological
  order at construction time. There is no separate `Validate()` a
  caller can forget to call; a `Workflow` value cannot exist in an
  invalid state.
- **Execution aggregate stays narrow**: `Execution` owns exactly
  lifecycle `State` and per-step `StepRun` progress. Budget ledger
  entries, policy decisions, and events reference an execution ID from
  their own owning packages rather than being embedded as fields here —
  "everything belongs to an execution" is a relationship, not a struct
  embedding. This avoids guessing at shapes (`internal/budget`,
  `internal/policy`, `internal/events`) that don't exist yet.
- **Repository pattern, in-memory today**: both packages define a
  `Repository` interface plus an `InMemoryRepository`, per the
  Milestone 1 decision that repository interfaces live next to their
  domain package and the Postgres implementation is a Milestone 7
  swap-in. `execution.InMemoryRepository` deliberately returns/stores
  **clones** (`Execution.Clone()`) rather than shared pointers, so code
  written against it can't rely on aliasing that won't hold once
  Milestone 7 swaps in Postgres — `Get()` gives you a snapshot; only
  `Update()` persists a change. This is the resolution (for the
  in-memory case) of open question #3 below on write concurrency: it
  doesn't provide cross-instance concurrency control, but it does
  enforce the same "read a copy, write explicitly" discipline the real
  implementation will need, so calling code is already written correctly
  against it.

### Execution state machine

The spec's own diagram renders the nine states as a single straight
line (`Retrying -> Completed -> Failed -> Cancelled`), which taken
literally would mean every execution retries, then completes, then
fails, then gets cancelled. Read in context it's clearly just an
enumeration of the state names, not a transition graph. The actual
graph implemented in `internal/execution/state.go`:

```mermaid
stateDiagram-v2
    [*] --> Created
    Created --> Queued
    Created --> Cancelled
    Queued --> Running
    Queued --> Cancelled
    Running --> Waiting
    Running --> Paused
    Running --> Retrying
    Running --> Completed
    Running --> Failed
    Running --> Cancelled
    Waiting --> Running
    Waiting --> Failed
    Waiting --> Cancelled
    Paused --> Running
    Paused --> Cancelled
    Retrying --> Running
    Retrying --> Failed
    Retrying --> Cancelled
    Completed --> [*]
    Failed --> [*]
    Cancelled --> [*]
```

Nothing generates this diagram from the code (or vice versa) — keep
them in sync by hand if the graph changes; `state.go`'s `transitions`
map is the authoritative source. `Completed`, `Failed`, and `Cancelled`
are terminal: `Execution.Transition` rejects any further transition
once one of those is reached, regardless of target.

Step-level progress (`StepRun.Status`) is intentionally a lighter
model: pending/running/waiting/completed/failed/skipped with only a
single rule enforced (a terminal step can't be modified further). It
does not have its own transition graph, and it does not decide *which*
step runs next based on the Workflow's dependency graph — that
ordering/dispatch logic belongs to the Scheduler / Execution Manager
(Milestone 5+), not to this aggregate.

## Milestone 3 — event bus and timeline engine

`internal/events` and `internal/timeline` are implemented. Key
decisions:

- **Store vs. Bus, as code**: this is the Milestone 1 architectural
  call (NATS = fan-out, Postgres = source of truth) made concrete.
  `events.Store` is the durable, replayable log (`InMemoryStore` today,
  Postgres in Milestone 7); `events.Bus` is best-effort live fan-out
  (`InMemoryBus` today — single-process pub/sub over channels, a slow
  subscriber gets dropped events rather than blocking the publisher). A
  NATS-backed `Bus` is deferred until something actually needs
  cross-instance delivery (multiple server replicas serving WebSocket
  subscribers) — the interface doesn't change either way.
- **`Recorder` ties them together**: producers (the not-yet-built
  Execution Manager, Policy Engine, Budget Engine) will depend on
  `events.Recorder.Record`, not wire `Store`+`Bus` separately. It
  appends first (durable), then publishes the exact stored copy —
  Sequence included — so a live subscriber and a later replay agree on
  ordering for the same event.
- **Sequence, not wall-clock time, defines order**: `Store.Append`
  assigns a `Sequence` monotonically per `ExecutionID`. Timestamps can
  collide or skew across processes; `timeline.Sort`/`Build` order by
  `Sequence`, never by `OccurredAt`.
- **Timeline stays decoupled from Execution/Workflow**: `internal/
  timeline` only imports `internal/events`. Human-readable labels (step
  name, tool name, policy name) travel in the event's own `Data`
  payload via well-known keys (`events.DataKeyStepName` etc.) rather
  than the timeline renderer reaching into a `Workflow` to look one up.
  This keeps rendering pure and trivially testable, at the cost of
  requiring producers to remember to populate those keys — worth
  revisiting if that turns out to be a common mistake once producers
  exist.
- **Replay semantics — resolved** (was open question #1): `timeline.
  Build` is a pure function over `events.Store.List`'s output, so
  calling it again on the same stored events reproduces an identical
  timeline. That *is* this milestone's implementation of "replay the
  execution timeline" — the safe, read-only sense. Re-executing a
  completed execution's side-effecting steps from some point (the
  other reading of "replay") is a different capability, is not what
  this implements, and remains explicitly out of scope until a later
  milestone deliberately designs for it (with the idempotency
  safeguards open question #2 below still describes).
- No HTTP/WebSocket exposure was added — `/events` and `/timeline`
  streaming endpoints are Gateway's job (Milestone 5), same scope
  discipline as Milestone 2 not adding REST routes for Execution/
  Workflow.

## Milestone 4 — budget engine and native policy engine

`internal/budget` and `internal/policy` are implemented. Budget was
folded into this milestone rather than left without a home — the
original plan named it as a Phase 1 deliverable but never gave it a
milestone slot, and Policy is specified to reference budget state, so
Budget had to exist first regardless.

- **The "Agent" gap, made explicit rather than silently worked around**:
  the spec's Domain Model section names Agent as a core concept ("owns
  credentials, policies, budgets, allowed tools") but the spec's own
  repository layout never lists an `internal/agent` package. Rather
  than invent a new top-level package the spec didn't ask for, or quietly
  ignore the concept, Budget and Policy both take a plain opaque
  `OwnerID`/`AgentID` string. This unblocks both packages today without
  guessing at what a full Agent aggregate (registration, credential
  storage, tool allowlists as data rather than passed-in config) should
  look like — that's Milestone 5's job, once Gateway needs agents to
  actually authenticate. Concretely, `policy.ToolAllowlistRule` takes a
  `map[string][]string` of agent ID → allowed tools as constructor
  config today; a real Agent registry would supply that same data
  later. **This means the spec's Phase 1 success criterion "Register an
  agent and its permissions" is still not implemented** — tracked
  explicitly rather than left to be discovered missing at the end.
- **Budget decides nothing, Policy decides everything**: `budget.Ledger`
  only tracks and reports (`Usage`, `Limit`, `Exceeded()`); whether an
  exceeded budget should deny an action is `policy.BudgetRule`'s job.
  This is the same boundary Milestone 2 drew around `Execution` (owns
  state, not decisions about that state).
- **Policy reads budget state through plain values, not a live
  pointer**: `policy.Input` carries `budget.Limit`/`budget.Usage`/`bool`
  fields, not a `*budget.Ledger`. A `Rule` could call `Ledger.Charge()`
  if handed the live aggregate — passing snapshotted values enforces
  "policy only reads budget" at the type level instead of by convention.
- **Deny-overrides evaluation, not first-match-wins**: `NativeEngine`
  evaluates every rule; any explicit deny wins immediately regardless of
  registration order, an explicit allow wins only if nothing denies, and
  the configured default effect applies if no rule has an opinion. This
  is the same model AWS IAM's explicit-deny and Kubernetes admission
  webhooks use, and it avoids a subtle bug class where an early
  permissive rule accidentally shadows a later, stricter one.
- **Four concrete rules ship**: `BudgetRule`, `ToolAllowlistRule`,
  `AllowedModelsRule`, `TimeWindowRule` — covering budget/agent/tool,
  provider/model, and time from the spec's named dimensions concretely.
  Workflow, execution, and environment are already present as
  `policy.Input` fields (plus an `Extra map[string]any` escape hatch)
  for a future rule to match on; there's no rule for them yet simply
  because nothing produces a meaningful policy for them today, not
  because the engine can't support it. "User role" is explicitly
  deferred in the spec itself.
- **The OPA/Cedar swap point is the `Engine` interface**:
  `policy.Engine` has one method, `Evaluate(ctx, Input) (Decision,
  error)`. `NativeEngine` is the only implementation today; a
  Rego/Cedar-backed one is additive later, same pattern as `Store`/`Bus`
  in Milestone 3 and `Repository` throughout.
- Same scope discipline as Milestones 2–3: no dependency on
  `internal/execution` or `internal/events` from either new package
  (verified: `internal/policy` imports only `internal/budget`;
  `internal/budget` imports no other `internal/*` package), and no HTTP
  exposure — a stored, named, enable/disable-able Policy record (the
  dashboard's "Policies: View, Enable, Disable, Test") is Milestone 5's
  job, layered on top of the `Rule`/`Engine` primitives built here.

## Milestone 5 — Gateway and protocol adapters

`internal/agent`, `internal/adapters` (+ `mcp`, `openai` subpackages),
and `internal/gateway` are implemented. This is the milestone where
every previously-isolated package finally gets composed — Workflow,
Execution, Events, Policy, Budget, and the new Adapters all get wired
together by `gateway.Service`, and the server gets its first real HTTP
surface.

- **Agent gets its package, resolving open question #6**: `internal/agent`
  is a new top-level package (not folded into `internal/auth` or
  `internal/gateway`) — the same reasoning that gave Workflow and
  Execution their own packages despite being referenced elsewhere.
  `internal/auth` remains reserved for control-plane-*operator*
  authN/authZ, a different concern from Agent-as-workload-identity.
  Only a salted hash of an issued bearer token is ever stored; the
  plaintext is returned exactly once, at registration.
- **`gateway.Service` is the spec's "Execution Manager"**: it's the
  first thing in this codebase allowed to import and compose
  `execution`, `workflow`, `events`, `policy`, `budget`, and `adapters`
  together — every one of those packages was kept deliberately
  ignorant of the others across Milestones 2-4 specifically so this
  composition would be possible without touching any of them. Its
  `run`/`runStep` driver is intentionally synchronous and sequential
  (`Workflow.TopologicalOrder()`, one step at a time, stop-on-first-
  failure) rather than the concurrent, backoff-and-retry-aware dispatch
  a real Scheduler would do — this proves the full wiring end-to-end;
  making it concurrent/resilient is additive future work, not a
  rewrite.
- **Adapters split by shape, not spec's list**: `adapters.Adapter`
  (tool calls) and `adapters.ModelAdapter` (model/chat calls) are two
  interfaces, not one — MCP is fundamentally about tool/resource
  invocation, OpenAI about chat completions, and forcing both into one
  interface would mean awkward unused fields either way.
- **MCP adapter is a real (if intentionally minimal) client**: it
  speaks actual JSON-RPC 2.0 over MCP's Streamable HTTP transport for
  `tools/call`, tested against an `httptest` server that speaks the
  same subset. What's missing relative to the full spec — initialize
  handshake, capability negotiation, `tools/list` discovery, SSE
  streaming, sessions — is a deliberate scope cut documented in the
  package itself, not a hidden gap.
- **OpenAI adapter makes real HTTP calls** to the Chat Completions
  endpoint (`net/http` directly, no SDK dependency) — single-turn,
  non-streaming, no function-calling. Tested against `httptest`, never
  the live API.
- **Policy reconciles a spec inconsistency**: the spec's one worked
  event-chain example says the event after `PolicyEvaluated` is
  `ToolApproved`, but its general event list says `PolicyApproved`.
  Milestone 3 had already standardized on `PolicyApproved`/
  `PolicyDenied` (they apply to model-call steps too, not just tool
  calls) — `gateway.Service` uses that pair, documented at the call
  site so the discrepancy isn't a silent surprise later.
- **No admin API for policy rules yet**: `internal/app` wires a
  `NativeEngine` with zero rules and `EffectAllow` as the default —
  the only honest choice until there's a way to configure real ones.
  Flagged explicitly (including in a code comment at the wiring site):
  a real deployment MUST configure real rules before this default
  matters. Building the stored, named, enable/disable-able Policy
  records the dashboard needs (Milestone 6) is what closes this gap.
- **Rate limiting**: Redis-backed fixed-window (`INCR`+`EXPIRE`),
  keyed by authenticated agent ID or source IP. Fails open on a Redis
  error — an unreachable limiter should degrade to unlimited, not take
  the API down; `/readyz` is where a Redis outage is supposed to
  surface, not request-time 500s.
- **WebSocket streaming has no history for late subscribers**: `GET
  .../executions/{id}/ws` streams live events from `events.Bus` only —
  Bus was deliberately built with no replay capability in Milestone 3.
  A client that needs full history should `GET .../events` first, then
  open the socket for what happens next; merging the two into one
  gapless stream is a real problem, left unsolved rather than partially
  solved here.
- **Agent registration is unauthenticated** — bootstrapping the very
  first agent would otherwise be circular, and there's no
  control-plane-operator auth (`internal/auth`) yet to gate it with.
  This is a known, flagged security gap for any deployment beyond local
  development, not an oversight.
- **Verified against a real running server**, not just unit tests:
  registered an agent, registered a workflow, started an execution, and
  watched it reach `completed` with the exact event chain from the
  spec's own example (`Execution Created → Started → StepStarted →
  PolicyEvaluated → PolicyApproved → StepCompleted → Execution
  Completed`) — over real HTTP, against real Postgres/Redis/NATS.

## Open questions carried over from spec review

Updated after Milestone 5 — #6 is resolved (see above), #2 is narrowed,
the rest are unchanged:

1. ~~Replay semantics~~ — resolved in Milestone 3.
2. **Tool-call idempotency** — still unresolved. `events.RetryScheduled`
   exists and adapters now make real calls (Milestone 5), so this is no
   longer blocked on anything else existing — it's just not designed
   yet. A retried step can still double-execute a side-effecting tool
   call today.
3. **Execution-write concurrency, distributed case** — the in-memory
   repository enforces copy-on-read/write discipline in-process (see
   Milestone 2 above), but single-writer-per-execution *across server
   instances* still needs a concrete mechanism (NATS subject keyed by
   execution ID, or Postgres optimistic locking via a version column)
   before Milestone 7's Postgres-backed `Repository` is implemented.
4. ~~Workflow definition format~~ — resolved in Milestone 2.
5. **Approval routing** — who approves an `Approval` step and what
   happens on timeout is still undefined. `StepTypeApproval` currently
   falls through `gateway.Service.runStep`'s no-adapter-yet default
   case (completes immediately as a no-op) — functionally fine for
   demoing wiring, but not a real approval flow.
6. ~~Agent identity/registration~~ — resolved in Milestone 5
   (`internal/agent`). Full credential lifecycle (rotation, revocation,
   a real secrets-manager/Vault/KMS integration) remains open, and
   agent registration being unauthenticated (see above) needs
   `internal/auth` before production use.
7. **No stored/configurable Policy records** — `NativeEngine` is wired
   with zero rules today; there's no API/dashboard path to add real
   ones yet. Needed before the default-allow posture is anything but a
   placeholder.
