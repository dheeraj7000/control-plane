"use client";

import Link from "next/link";
import { useState } from "react";
import { useExecutions, useWorkflows } from "@/hooks/use-api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select } from "@/components/ui/select";
import { ExecutionStateBadge } from "@/components/status-badge";
import type { ExecutionState } from "@/lib/types";

const STATES: ExecutionState[] = [
  "created",
  "queued",
  "running",
  "waiting",
  "paused",
  "retrying",
  "completed",
  "failed",
  "cancelled",
];

export function ExecutionTable() {
  const [workflowId, setWorkflowId] = useState("");
  const [state, setState] = useState<ExecutionState | "">("");
  const { data: workflows } = useWorkflows();
  const { data, isLoading, isError, error } = useExecutions({ workflowId, state });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex gap-2">
        <Select value={workflowId} onChange={(e) => setWorkflowId(e.target.value)} className="max-w-56">
          <option value="">All workflows</option>
          {workflows?.map((wf) => (
            <option key={wf.id} value={wf.id}>
              {wf.name} ({wf.id})
            </option>
          ))}
        </Select>
        <Select value={state} onChange={(e) => setState(e.target.value as ExecutionState | "")} className="max-w-40">
          <option value="">All states</option>
          {STATES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </Select>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading executions…</p>}
      {isError && <p className="text-sm text-destructive">{(error as Error).message}</p>}
      {!isLoading && !data?.length && <p className="text-sm text-muted-foreground">No executions match.</p>}

      {!!data?.length && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>Workflow</TableHead>
              <TableHead>Agent</TableHead>
              <TableHead>State</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.map((exec) => (
              <TableRow key={exec.id}>
                <TableCell className="font-mono text-xs">
                  <Link href={`/executions/${encodeURIComponent(exec.id)}`} className="underline underline-offset-2">
                    {exec.id}
                  </Link>
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {exec.workflow_id}@{exec.workflow_version}
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  {exec.agent_id || "—"}
                </TableCell>
                <TableCell>
                  <ExecutionStateBadge state={exec.state} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
