import { Badge } from "@/components/ui/badge";
import type { ExecutionState, StepStatus } from "@/lib/types";

const EXECUTION_VARIANT: Record<ExecutionState, "default" | "secondary" | "success" | "warning" | "destructive"> = {
  created: "secondary",
  queued: "secondary",
  running: "warning",
  waiting: "warning",
  paused: "warning",
  retrying: "warning",
  completed: "success",
  failed: "destructive",
  cancelled: "destructive",
};

export function ExecutionStateBadge({ state }: { state: ExecutionState }) {
  return <Badge variant={EXECUTION_VARIANT[state] ?? "default"}>{state}</Badge>;
}

const STEP_VARIANT: Record<StepStatus, "default" | "secondary" | "success" | "warning" | "destructive"> = {
  pending: "secondary",
  running: "warning",
  waiting: "warning",
  completed: "success",
  failed: "destructive",
  skipped: "secondary",
};

export function StepStatusBadge({ status }: { status: StepStatus }) {
  return <Badge variant={STEP_VARIANT[status] ?? "default"}>{status}</Badge>;
}
