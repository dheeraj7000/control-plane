"use client";

import { useState } from "react";
import { AgentTable } from "@/components/agents/agent-table";
import { RegisterAgentForm } from "@/components/agents/register-agent-form";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";

export default function AgentsPage() {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Agents</h1>
          <p className="text-sm text-muted-foreground">
            Identities that authenticate to this control plane — each has an allowed-tools list and a
            bearer token (see internal/agent).
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>Register agent</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Register a new agent</DialogTitle>
            </DialogHeader>
            <RegisterAgentForm />
          </DialogContent>
        </Dialog>
      </div>
      <AgentTable />
    </div>
  );
}
