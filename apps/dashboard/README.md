# dashboard

Next.js + React + TypeScript control plane dashboard — Milestone 6.
Renders exactly what the REST/WebSocket API (Milestones 3–5) exposes,
nothing more: Overview, Agents, Workflows (+ step-graph detail),
Executions (+ live detail: step graph, timeline, raw events, budget),
Settings.

## Stack

Per the spec's planned stack, with a couple of deliberate substitutions:

| Named in spec       | Used here                       | Why |
|---------------------|----------------------------------|-----|
| Next.js, React, TS  | Next.js 16 (App Router), React 19, TS | as specified |
| Tailwind            | Tailwind CSS v4                 | as specified |
| shadcn/ui           | hand-rolled primitives (`components/ui/*`) using shadcn's token names/visual language, plus real Radix primitives (`@radix-ui/react-dialog`, `-tabs`) for the two components that need real accessible behavior | the shadcn CLI vendors generated component source into the repo from a registry; hand-rolling a small primitive set gets the same look without that indirection, for a component surface this small |
| React Flow          | `@xyflow/react`                 | React Flow's package moved to this scoped name |
| TanStack Query      | `@tanstack/react-query` v5      | as specified — all server state |
| Zustand             | `zustand`                        | as specified — just the auth/session store, see `lib/store.ts` |
| Recharts            | `recharts`                       | as specified — one chart (execution-state breakdown), computed client-side over whatever the Executions list query already fetched (no metrics/aggregation endpoint exists on the backend) |
| Framer Motion       | `framer-motion`                  | as specified, used deliberately sparingly — one page-transition fade, see `components/layout/page-transition.tsx`'s comment for why |

## What's cut, and why

This dashboard renders every route `internal/gateway` actually
exposes. It does **not** have Policies, Tool Registry, or Model Router
pages — those would need admin APIs that don't exist yet (see
`docs/architecture.md`'s open questions #7: "No stored/configurable
Policy records"). Building an admin UI for state the backend can't
persist would just be a mockup, not a working feature — the honest
scope for this milestone is "dashboard for what Milestones 3–5 built,"
not "every page the spec's dashboard section describes."

Workflow authoring is a raw JSON textarea, not a visual step builder —
`internal/workflow.Workflow`'s `UnmarshalJSON` already gives pasted
JSON the exact same validation (unique step IDs, resolvable
dependencies, acyclic graph) a Go caller gets, so this is a real,
fully-validated path to registering a workflow, just not a drag-and-drop
one.

The workflow DAG visualization uses a simple longest-path layered
layout (see `lib/workflow-graph.ts`), not a general graph-drawing
algorithm — fine for the shallow DAGs this system deals with in
practice.

## Auth

There is no control-plane-operator login — `internal/auth` is still
unbuilt (see `docs/architecture.md`'s open questions). The dashboard
authenticates as an **agent**, the same credential a programmatic
caller would use: register one from the Settings page (or Agents page)
and its bearer token is stored in `localStorage` via the Zustand store
in `lib/store.ts`, then sent as an `Authorization: Bearer` header on
every request (see `lib/api.ts`).

The one exception is `GET .../executions/{id}/ws`: browsers cannot set
an `Authorization` header on a WebSocket handshake, so that one route
accepts the token as a `?token=` query parameter instead — a narrow,
documented backend change (`internal/gateway/auth.go`'s
`AuthMiddlewareWS`), not a general relaxation of the API's auth scheme.

## Local development

```bash
# 1. Run the API server (from the repo root)
make dev-up && make run

# 2. In another terminal, run the dashboard
cd apps/dashboard
echo "NEXT_PUBLIC_API_BASE_URL=http://localhost:8080" > .env.local
npm install
npm run dev
```

Then open http://localhost:3000, go to Settings, and register an agent
(or paste an existing one's token) to authenticate.

Or run everything, dashboard included, via `make docker-up` from the
repo root — see `deployments/dashboard.Dockerfile`.

## Structure

```
app/            Next.js App Router pages
components/
  ui/           hand-rolled primitives (Button, Card, Table, Dialog, Tabs, ...)
  agents/       Agents page's table + register form
  workflows/    Workflows list/detail, DAG visualization, register form
  executions/   Executions list/detail, timeline, raw events, budget panel
  overview/     Overview page's stat cards + chart
  layout/       Sidebar, header, page-transition shell
hooks/          TanStack Query hooks (one per resource) + the WebSocket hook
lib/            API client, types mirroring the Go JSON wire shapes, Zustand store, utilities
```
