"use client";

import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { Execution, ExecutionState } from "@/lib/types";

const ORDER: ExecutionState[] = [
  "running",
  "queued",
  "created",
  "waiting",
  "paused",
  "retrying",
  "completed",
  "failed",
  "cancelled",
];

// A client-computed aggregate over whatever `executions` the caller
// already fetched (see app/page.tsx) — there's no metrics/aggregation
// endpoint on the backend (see docs/architecture.md's cut scope for
// this milestone), so this is exactly as accurate as the list it's
// derived from, not a server-computed rollup.
export function StateChart({ executions }: { executions: Execution[] }) {
  const counts = new Map<ExecutionState, number>();
  for (const e of executions) counts.set(e.state, (counts.get(e.state) ?? 0) + 1);
  const data = ORDER.filter((s) => counts.get(s)).map((state) => ({ state, count: counts.get(state) }));

  if (!data.length) {
    return <p className="text-sm text-muted-foreground">No executions yet.</p>;
  }

  return (
    <ResponsiveContainer width="100%" height={220}>
      <BarChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
        <XAxis dataKey="state" tick={{ fontSize: 12 }} stroke="var(--color-muted-foreground)" />
        <YAxis allowDecimals={false} tick={{ fontSize: 12 }} stroke="var(--color-muted-foreground)" />
        <Tooltip
          contentStyle={{
            background: "var(--color-card)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            fontSize: 12,
          }}
        />
        <Bar dataKey="count" fill="var(--color-primary)" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}
