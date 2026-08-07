"use client";

import { useState } from "react";
import { useRegisterAgent } from "@/hooks/use-api";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { useSession } from "@/lib/store";
import type { RegisterAgentResponse } from "@/lib/types";

// RegisterAgentForm is shared between the Agents page's "Register
// agent" dialog and the Settings page's bootstrap flow — both are
// exactly the same POST /agents call (deliberately unauthenticated on
// the backend, see internal/gateway/http.go's Mount comment: there's
// no control-plane-operator auth yet to gate who may register the
// very first agent).
export function RegisterAgentForm({ onRegistered }: { onRegistered?: (r: RegisterAgentResponse) => void }) {
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [tools, setTools] = useState("");
  const [result, setResult] = useState<RegisterAgentResponse | null>(null);
  const register = useRegisterAgent();
  const setCredentials = useSession((s) => s.setCredentials);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const allowedTools = tools
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);
    const res = await register.mutateAsync({ id, name, allowedTools });
    setResult(res);
    onRegistered?.(res);
  }

  if (result) {
    return (
      <div className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground">
          Agent <span className="font-medium text-foreground">{result.agent.id}</span> registered.
          Its bearer token is shown <span className="font-medium text-foreground">once</span> — copy it now
          (internal/agent only ever stores a hash, see docs/architecture.md).
        </p>
        <code className="break-all rounded-md border border-border bg-muted p-3 text-xs">{result.token}</code>
        <Button
          type="button"
          onClick={() => setCredentials(result.agent.id, result.agent.name, result.token)}
        >
          Use these credentials for this dashboard
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="agent-id">Agent ID</Label>
        <Input id="agent-id" required value={id} onChange={(e) => setId(e.target.value)} placeholder="agent-search-bot" />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="agent-name">Name</Label>
        <Input id="agent-name" required value={name} onChange={(e) => setName(e.target.value)} placeholder="Search Bot" />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="agent-tools">Allowed tools (comma-separated)</Label>
        <Input id="agent-tools" value={tools} onChange={(e) => setTools(e.target.value)} placeholder="search, github.read" />
      </div>
      {register.isError && (
        <p className="text-sm text-destructive">{(register.error as Error).message}</p>
      )}
      <Button type="submit" disabled={register.isPending}>
        {register.isPending ? "Registering…" : "Register agent"}
      </Button>
    </form>
  );
}
