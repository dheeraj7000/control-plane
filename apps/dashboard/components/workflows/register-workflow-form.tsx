"use client";

import { useState } from "react";
import { useRegisterWorkflow } from "@/hooks/use-api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/input";

const TEMPLATE = `{
  "id": "example-workflow",
  "name": "Example Workflow",
  "version": 1,
  "description": "Search then summarize",
  "steps": [
    { "id": "search", "type": "search", "name": "Search" },
    { "id": "summarize", "type": "summarize", "name": "Summarize", "depends_on": ["search"] }
  ]
}`;

// A raw JSON textarea, not a visual step builder — the spec names a
// dashboard-authored workflow editor as later, more ambitious work.
// internal/workflow.Workflow's UnmarshalJSON already gives this the
// exact same validation (unique step IDs, resolvable dependencies,
// acyclic graph) a Go-side caller gets, so pasted JSON that would be
// rejected here is genuinely invalid, not just rejected by a
// client-side form the server would have accepted.
export function RegisterWorkflowForm({ onRegistered }: { onRegistered?: () => void }) {
  const [text, setText] = useState(TEMPLATE);
  const [parseError, setParseError] = useState<string | null>(null);
  const register = useRegisterWorkflow();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setParseError(null);
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch (err) {
      setParseError((err as Error).message);
      return;
    }
    await register.mutateAsync(parsed);
    onRegistered?.();
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      <Textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={14}
        spellCheck={false}
      />
      {parseError && <p className="text-sm text-destructive">Invalid JSON: {parseError}</p>}
      {register.isError && (
        <p className="text-sm text-destructive">{(register.error as Error).message}</p>
      )}
      {register.isSuccess && <p className="text-sm text-success">Workflow registered.</p>}
      <Button type="submit" disabled={register.isPending}>
        {register.isPending ? "Registering…" : "Register workflow"}
      </Button>
    </form>
  );
}
