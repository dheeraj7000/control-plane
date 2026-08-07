"use client";

import { useBudget } from "@/hooks/use-api";
import { Badge } from "@/components/ui/badge";
import { formatUsd } from "@/lib/utils";

function bar(used: number, limit: number | undefined) {
  if (!limit) return null;
  const pct = Math.min(100, Math.round((used / limit) * 100));
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
      <div
        className={pct >= 100 ? "h-full bg-destructive" : "h-full bg-primary"}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

export function BudgetPanel({ executionId, live }: { executionId: string; live: boolean }) {
  const { data, isLoading, isError, error } = useBudget(executionId, live);

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading budget…</p>;
  if (isError) return <p className="text-sm text-destructive">{(error as Error).message}</p>;
  if (!data) return null;

  return (
    <div className="flex flex-col gap-4">
      {data.exceeded && <Badge variant="destructive">Budget exceeded</Badge>}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="flex flex-col gap-1">
          <span className="text-xs text-muted-foreground">Input tokens</span>
          <span className="text-lg font-semibold">{data.input_tokens.toLocaleString()}</span>
          {bar(data.input_tokens, data.limit_input_tokens)}
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-muted-foreground">Output tokens</span>
          <span className="text-lg font-semibold">{data.output_tokens.toLocaleString()}</span>
          {bar(data.output_tokens, data.limit_output_tokens)}
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-muted-foreground">Cost</span>
          <span className="text-lg font-semibold">{formatUsd(data.cost_usd)}</span>
          {bar(data.cost_usd, data.limit_cost_usd)}
        </div>
      </div>
      {!data.limit_input_tokens && !data.limit_output_tokens && !data.limit_cost_usd && (
        <p className="text-xs text-muted-foreground">
          No limit configured for this execution — internal/app wires DefaultExecutionBudget to
          Limit{"{}"} (unlimited) until a deployment sets one.
        </p>
      )}
    </div>
  );
}
