"use client";

import Link from "next/link";
import { useWorkflows } from "@/hooks/use-api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { formatDateTime } from "@/lib/utils";

export function WorkflowTable() {
  const { data, isLoading, isError, error } = useWorkflows();

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading workflows…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!data?.length) return <p className="text-sm text-muted-foreground">No workflows registered yet.</p>;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Version</TableHead>
          <TableHead>Steps</TableHead>
          <TableHead>Created</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {data.map((wf) => (
          <TableRow key={`${wf.id}@${wf.version}`}>
            <TableCell className="font-mono text-xs">
              <Link href={`/workflows/${encodeURIComponent(wf.id)}`} className="underline underline-offset-2">
                {wf.id}
              </Link>
            </TableCell>
            <TableCell>{wf.name}</TableCell>
            <TableCell>{wf.version}</TableCell>
            <TableCell>{wf.steps.length}</TableCell>
            <TableCell className="text-muted-foreground">{formatDateTime(wf.created_at)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
