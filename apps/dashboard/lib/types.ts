// Wire types mirroring the Go API's JSON response shapes exactly
// (internal/gateway/http.go's view types, internal/workflow's DTO,
// internal/events.Event, internal/timeline.Entry). Kept hand-written
// rather than generated from api/openapi.yaml — the spec named an
// OpenAPI-generated SDK (sdk/) as later work; this milestone is about
// a working dashboard against the real API, not a codegen pipeline.

export type Agent = {
  id: string;
  name: string;
  allowed_tools?: string[];
};

export type RegisterAgentResponse = {
  agent: Agent;
  token: string;
};

export type StepType =
  | "search"
  | "summarize"
  | "call_tool"
  | "review"
  | "wait"
  | "approval"
  | "model_call";

export type Step = {
  id: string;
  name?: string;
  type: StepType;
  depends_on?: string[];
  config?: Record<string, unknown>;
};

export type Workflow = {
  id: string;
  name: string;
  version: number;
  description?: string;
  steps: Step[];
  metadata?: Record<string, string>;
  created_at: string;
};

// The nine states an Execution can be in — internal/execution.State's
// closed set. Kept as a union here (not a Go-generated enum) so the
// dashboard's TypeScript catches a typo against the same list the
// backend validates.
export type ExecutionState =
  | "created"
  | "queued"
  | "running"
  | "waiting"
  | "paused"
  | "retrying"
  | "completed"
  | "failed"
  | "cancelled";

export type Execution = {
  id: string;
  workflow_id: string;
  workflow_version: number;
  agent_id?: string;
  state: ExecutionState;
};

export type EventType =
  | "execution.created"
  | "execution.started"
  | "execution.paused"
  | "execution.completed"
  | "execution.failed"
  | "execution.cancelled"
  | "step.started"
  | "step.completed"
  | "step.failed"
  | "tool.requested"
  | "tool.executed"
  | "tool.failed"
  | "policy.evaluated"
  | "policy.approved"
  | "policy.denied"
  | "budget.updated"
  | "budget.exceeded"
  | "retry.scheduled";

export type ControlPlaneEvent = {
  id: string;
  execution_id: string;
  type: EventType;
  occurred_at: string;
  sequence: number;
  data?: Record<string, unknown>;
};

export type TimelineEntry = {
  event_id: string;
  execution_id: string;
  sequence: number;
  at: string;
  label: string;
  detail: string;
  type: EventType;
};

export type Budget = {
  scope: "execution" | "daily" | "monthly";
  owner_id: string;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
  limit_input_tokens?: number;
  limit_output_tokens?: number;
  limit_cost_usd?: number;
  exceeded: boolean;
};

// StepStatus mirrors internal/execution.StepStatus — but there's no
// API field carrying it directly (executionView deliberately doesn't
// expose the Steps map, see docs/architecture.md's Milestone 6 notes).
// The dashboard derives this per step from the event stream instead;
// see lib/step-status.ts.
export type StepStatus =
  | "pending"
  | "running"
  | "waiting"
  | "completed"
  | "failed"
  | "skipped";

export type ApiError = {
  error: string;
};
