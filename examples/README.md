# examples

- `research-workflow.json` — a three-step Workflow definition (search →
  summarize → human approval) in the canonical JSON format decided in
  Milestone 2 (see `docs/architecture.md`). It's not just documentation:
  `internal/workflow`'s tests load this exact file to prove
  `Workflow.UnmarshalJSON` parses and validates it.

More end-to-end samples ("register an agent, run a workflow through the
MCP adapter, watch its timeline") land once there's a running system to
demonstrate — Milestone 5 onward.
