import type { ControlPlaneEvent, StepStatus } from "./types";

// deriveStepStatuses reconstructs each step's execution.StepStatus
// from the raw event stream instead of the API exposing it directly.
//
// Why derive rather than add a field: internal/gateway's
// executionView deliberately hand-picks a small set of fields (id,
// workflow_id, workflow_version, agent_id, state) rather than
// exposing execution.Execution's internal Steps map — see
// docs/architecture.md. Every step-status transition this package
// cares about (StepStarted/StepCompleted/StepFailed) is already a
// distinct, durably recorded event carrying step_id in its Data
// payload (internal/gateway/service.go's runStep/failStep), so a pure
// client-side projection over `GET .../events` is a complete answer
// without a backend change — the same "replay is a pure projection"
// principle internal/timeline.Build already relies on.
export function deriveStepStatuses(
  events: ControlPlaneEvent[],
): Map<string, StepStatus> {
  const statuses = new Map<string, StepStatus>();
  const sorted = [...events].sort((a, b) => a.sequence - b.sequence);

  for (const e of sorted) {
    const stepId = e.data?.["step_id"];
    if (typeof stepId !== "string") continue;

    switch (e.type) {
      case "step.started":
        statuses.set(stepId, "running");
        break;
      case "step.completed":
        statuses.set(stepId, "completed");
        break;
      case "step.failed":
        statuses.set(stepId, "failed");
        break;
    }
  }
  return statuses;
}

export function stepStatusOf(
  statuses: Map<string, StepStatus>,
  stepId: string,
): StepStatus {
  return statuses.get(stepId) ?? "pending";
}
