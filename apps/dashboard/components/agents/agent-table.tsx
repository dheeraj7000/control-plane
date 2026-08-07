"use client";

import { useAgents } from "@/hooks/use-api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

export function AgentTable() {
  const { data, isLoading, isError, error } = useAgents();

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading agents…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!data?.length) return <p className="text-sm text-muted-foreground">No agents registered yet.</p>;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>Name</TableHead>
          <TableHead>Allowed tools</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {data.map((agent) => (
          <TableRow key={agent.id}>
            <TableCell className="font-mono text-xs">{agent.id}</TableCell>
            <TableCell>{agent.name}</TableCell>
            <TableCell>
              {agent.allowed_tools?.length ? (
                <div className="flex flex-wrap gap-1">
                  {agent.allowed_tools.map((t) => (
                    <Badge key={t} variant="outline">{t}</Badge>
                  ))}
                </div>
              ) : (
                <span className="text-xs text-muted-foreground">all tools</span>
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
