"use client";

import { useState } from "react";
import { WorkflowTable } from "@/components/workflows/workflow-table";
import { RegisterWorkflowForm } from "@/components/workflows/register-workflow-form";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";

export default function WorkflowsPage() {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Workflows</h1>
          <p className="text-sm text-muted-foreground">
            Immutable templates — starting an execution runs the latest registered version.
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>Register workflow</Button>
          </DialogTrigger>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>Register a new workflow</DialogTitle>
            </DialogHeader>
            <RegisterWorkflowForm onRegistered={() => setOpen(false)} />
          </DialogContent>
        </Dialog>
      </div>
      <WorkflowTable />
    </div>
  );
}
