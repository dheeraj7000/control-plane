"use client";

import Link from "next/link";
import { useAgents, useWorkflows, useExecutions } from "@/hooks/use-api";
import { StatCard } from "@/components/overview/stat-card";
import { StateChart } from "@/components/overview/state-chart";
import { ExecutionStateBadge } from "@/components/status-badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export default function OverviewPage() {
  const { data: agents } = useAgents();
  const { data: workflows } = useWorkflows();
  const { data: executions } = useExecutions();

  const running = executions?.filter((e) => e.state === "running" || e.state === "queued").length ?? 0;
  const recent = (executions ?? []).slice(-10).reverse();

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-lg font-semibold">Overview</h1>

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatCard label="Agents" value={agents?.length ?? "—"} />
        <StatCard label="Workflows" value={workflows?.length ?? "—"} />
        <StatCard label="Executions" value={executions?.length ?? "—"} />
        <StatCard label="Running now" value={running} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Executions by state</CardTitle>
        </CardHeader>
        <CardContent>
          <StateChart executions={executions ?? []} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Recent executions</CardTitle>
        </CardHeader>
        <CardContent>
          {recent.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Nothing yet — register an agent and a workflow, then start an execution.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Workflow</TableHead>
                  <TableHead>State</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recent.map((exec) => (
                  <TableRow key={exec.id}>
                    <TableCell className="font-mono text-xs">
                      <Link href={`/executions/${encodeURIComponent(exec.id)}`} className="underline underline-offset-2">
                        {exec.id}
                      </Link>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">{exec.workflow_id}</TableCell>
                    <TableCell>
                      <ExecutionStateBadge state={exec.state} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
