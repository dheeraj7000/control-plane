"use client";

import { useTimeline } from "@/hooks/use-api";
import { formatDateTime } from "@/lib/utils";

// Renders exactly the spec's own worked example shape:
//   14:02  Execution Started
//   14:06  Budget      +3500 Tokens
// — internal/timeline.Entry already carries Label/Detail split for
// this, so there's no rendering logic here beyond the split itself.
export function TimelineList({ executionId, live }: { executionId: string; live: boolean }) {
  const { data, isLoading, isError, error } = useTimeline(executionId, live);

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading timeline…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!data?.length) return <p className="text-sm text-muted-foreground">No events recorded yet.</p>;

  return (
    <ol className="flex flex-col gap-2">
      {data.map((entry) => (
        <li key={entry.event_id} className="flex items-baseline gap-3 border-l-2 border-border pl-3 text-sm">
          <span className="w-32 shrink-0 font-mono text-xs text-muted-foreground">
            {formatDateTime(entry.at)}
          </span>
          <span className="font-medium">{entry.label}</span>
          <span className="text-muted-foreground">{entry.detail}</span>
        </li>
      ))}
    </ol>
  );
}
