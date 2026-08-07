"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useWorkflows, useStartExecution } from "@/hooks/use-api";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Label } from "@/components/ui/input";

export function StartExecutionForm({ onStarted }: { onStarted?: () => void }) {
  const { data: workflows } = useWorkflows();
  const [workflowId, setWorkflowId] = useState("");
  const startExecution = useStartExecution();
  const router = useRouter();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!workflowId) return;
    const exec = await startExecution.mutateAsync(workflowId);
    onStarted?.();
    router.push(`/executions/${encodeURIComponent(exec.id)}`);
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="workflow-select">Workflow</Label>
        <Select id="workflow-select" required value={workflowId} onChange={(e) => setWorkflowId(e.target.value)}>
          <option value="" disabled>
            Choose a workflow…
          </option>
          {workflows?.map((wf) => (
            <option key={wf.id} value={wf.id}>
              {wf.name} ({wf.id}) — v{wf.version}
            </option>
          ))}
        </Select>
      </div>
      {startExecution.isError && (
        <p className="text-sm text-destructive">{(startExecution.error as Error).message}</p>
      )}
      <Button type="submit" disabled={startExecution.isPending || !workflowId}>
        {startExecution.isPending ? "Starting…" : "Start execution"}
      </Button>
    </form>
  );
}
