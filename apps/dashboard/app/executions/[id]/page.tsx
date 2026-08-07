"use client";

import { use, useMemo } from "react";
import Link from "next/link";
import { useExecution, useWorkflow, useEvents, useExecutionSocket } from "@/hooks/use-api";
import { ExecutionStateBadge } from "@/components/status-badge";
import { WorkflowGraph } from "@/components/workflows/workflow-graph";
import { TimelineList } from "@/components/executions/timeline-list";
import { EventsTable } from "@/components/executions/events-table";
import { BudgetPanel } from "@/components/executions/budget-panel";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { deriveStepStatuses } from "@/lib/step-status";
import type { ExecutionState } from "@/lib/types";

function isTerminal(state: ExecutionState | undefined) {
  return state === "completed" || state === "failed" || state === "cancelled";
}

export default function ExecutionDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: exec, isLoading, isError, error } = useExecution(id);
  const live = !isTerminal(exec?.state);

  // The workflow lookup here is always the *latest* registered
  // version (GET /workflows/{id} — see internal/gateway/http.go,
  // there's no route exposing an exact historical version even
  // though gateway.Service.GetWorkflow supports one). Fine for
  // rendering the step graph in practice — workflow definitions are
  // small and versions rarely reshape the DAG — but worth knowing
  // this can show a graph that's drifted from exec.workflow_version.
  const { data: wf } = useWorkflow(exec?.workflow_id ?? "");
  const { data: events } = useEvents(id, live);
  useExecutionSocket(id, live);

  const statuses = useMemo(() => deriveStepStatuses(events ?? []), [events]);

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading execution…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!exec) return null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="font-mono text-lg font-semibold">{exec.id}</h1>
          <p className="text-sm text-muted-foreground">
            <Link href={`/workflows/${encodeURIComponent(exec.workflow_id)}`} className="underline underline-offset-2">
              {exec.workflow_id}
            </Link>{" "}
            @ v{exec.workflow_version}
            {exec.agent_id && <> · agent {exec.agent_id}</>}
          </p>
        </div>
        <ExecutionStateBadge state={exec.state} />
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="timeline">Timeline</TabsTrigger>
          <TabsTrigger value="events">Raw events</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="flex flex-col gap-4">
          {wf && (
            <Card>
              <CardHeader>
                <CardTitle>Step graph</CardTitle>
              </CardHeader>
              <CardContent>
                <WorkflowGraph steps={wf.steps} statuses={statuses} />
              </CardContent>
            </Card>
          )}
          <Card>
            <CardHeader>
              <CardTitle>Budget</CardTitle>
            </CardHeader>
            <CardContent>
              <BudgetPanel executionId={id} live={live} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="timeline">
          <Card>
            <CardContent className="pt-4">
              <TimelineList executionId={id} live={live} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="events">
          <Card>
            <CardContent className="pt-4">
              <EventsTable executionId={id} live={live} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
