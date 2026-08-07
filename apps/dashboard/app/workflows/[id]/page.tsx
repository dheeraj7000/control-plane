"use client";

import { use, useState } from "react";
import { useRouter } from "next/navigation";
import { useWorkflow, useStartExecution } from "@/hooks/use-api";
import { WorkflowGraph } from "@/components/workflows/workflow-graph";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatDateTime } from "@/lib/utils";

export default function WorkflowDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: wf, isLoading, isError, error } = useWorkflow(id);
  const startExecution = useStartExecution();
  const router = useRouter();
  const [startError, setStartError] = useState<string | null>(null);

  async function handleStart() {
    if (!wf) return;
    setStartError(null);
    try {
      const exec = await startExecution.mutateAsync(wf.id);
      router.push(`/executions/${encodeURIComponent(exec.id)}`);
    } catch (err) {
      setStartError((err as Error).message);
    }
  }

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading workflow…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!wf) return null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-lg font-semibold">{wf.name}</h1>
          <p className="font-mono text-xs text-muted-foreground">
            {wf.id} · v{wf.version} · created {formatDateTime(wf.created_at)}
          </p>
          {wf.description && <p className="mt-1 text-sm text-muted-foreground">{wf.description}</p>}
        </div>
        <div className="flex flex-col items-end gap-1">
          <Button onClick={handleStart} disabled={startExecution.isPending}>
            {startExecution.isPending ? "Starting…" : "Start execution"}
          </Button>
          {startError && <p className="text-xs text-destructive">{startError}</p>}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Step graph</CardTitle>
        </CardHeader>
        <CardContent>
          <WorkflowGraph steps={wf.steps} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Steps</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Depends on</TableHead>
                <TableHead>Config</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {wf.steps.map((step) => (
                <TableRow key={step.id}>
                  <TableCell className="font-mono text-xs">{step.id}</TableCell>
                  <TableCell>{step.name || "—"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{step.type}</Badge>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {step.depends_on?.join(", ") || "—"}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {step.config ? JSON.stringify(step.config) : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
