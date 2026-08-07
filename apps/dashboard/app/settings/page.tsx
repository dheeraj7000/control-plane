"use client";

import { useState } from "react";
import { useSession } from "@/lib/store";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { RegisterAgentForm } from "@/components/agents/register-agent-form";

export default function SettingsPage() {
  const { apiBaseUrl, agentId, agentName, token, setApiBaseUrl, setCredentials, clearCredentials } = useSession();
  const [urlDraft, setUrlDraft] = useState(apiBaseUrl);
  const [pasteId, setPasteId] = useState("");
  const [pasteName, setPasteName] = useState("");
  const [pasteToken, setPasteToken] = useState("");

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <h1 className="text-lg font-semibold">Settings</h1>

      <Card>
        <CardHeader>
          <CardTitle>API connection</CardTitle>
          <CardDescription>Where the dashboard sends every request (NEXT_PUBLIC_API_BASE_URL by default).</CardDescription>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Input value={urlDraft} onChange={(e) => setUrlDraft(e.target.value)} placeholder="http://localhost:8080" />
          <Button onClick={() => setApiBaseUrl(urlDraft)}>Save</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Credentials</CardTitle>
          <CardDescription>
            There is no control-plane-operator login yet (internal/auth is still unbuilt — see
            docs/architecture.md). The dashboard authenticates as an agent, the same as any programmatic
            caller.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {token ? (
            <div className="flex items-center justify-between rounded-md border border-border p-3 text-sm">
              <span>
                Authenticated as <span className="font-medium">{agentName}</span>{" "}
                <span className="font-mono text-xs text-muted-foreground">({agentId})</span>
              </span>
              <Button variant="outline" size="sm" onClick={clearCredentials}>
                Clear
              </Button>
            </div>
          ) : (
            <p className="text-sm text-warning">Not authenticated — most routes will 401.</p>
          )}

          <div>
            <h3 className="mb-2 text-sm font-medium">Use an existing agent&apos;s token</h3>
            <div className="flex flex-col gap-2">
              <Input placeholder="Agent ID" value={pasteId} onChange={(e) => setPasteId(e.target.value)} />
              <Input placeholder="Agent name (for display only)" value={pasteName} onChange={(e) => setPasteName(e.target.value)} />
              <Input placeholder="Bearer token" value={pasteToken} onChange={(e) => setPasteToken(e.target.value)} />
              <Button
                variant="outline"
                disabled={!pasteId || !pasteToken}
                onClick={() => setCredentials(pasteId, pasteName || pasteId, pasteToken)}
              >
                Use these credentials
              </Button>
            </div>
          </div>

          <div>
            <h3 className="mb-2 text-sm font-medium">Or register a new agent</h3>
            <RegisterAgentForm />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
