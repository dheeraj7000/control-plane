"use client";

import { useEvents } from "@/hooks/use-api";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/utils";

// The raw event log, one row per events.Event — the audit-trail view
// alongside TimelineList's human-readable projection of the same
// data. Both read GET .../events / .../timeline independently rather
// than one deriving from the other client-side, matching the backend:
// internal/timeline.Build projects the Store's events, but the
// dashboard doesn't need to re-implement that projection just because
// it already has the raw events in hand for this tab.
export function EventsTable({ executionId, live }: { executionId: string; live: boolean }) {
  const { data, isLoading, isError, error } = useEvents(executionId, live);

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading events…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!data?.length) return <p className="text-sm text-muted-foreground">No events recorded yet.</p>;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Seq</TableHead>
          <TableHead>Type</TableHead>
          <TableHead>At</TableHead>
          <TableHead>Data</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {[...data]
          .sort((a, b) => a.sequence - b.sequence)
          .map((evt) => (
            <TableRow key={evt.id}>
              <TableCell className="font-mono text-xs">{evt.sequence}</TableCell>
              <TableCell>
                <Badge variant="outline">{evt.type}</Badge>
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">{formatDateTime(evt.occurred_at)}</TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                {evt.data && Object.keys(evt.data).length ? JSON.stringify(evt.data) : "—"}
              </TableCell>
            </TableRow>
          ))}
      </TableBody>
    </Table>
  );
}
