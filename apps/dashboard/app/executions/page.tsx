"use client";

import { useState } from "react";
import { ExecutionTable } from "@/components/executions/execution-table";
import { StartExecutionForm } from "@/components/executions/start-execution-form";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";

export default function ExecutionsPage() {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Executions</h1>
          <p className="text-sm text-muted-foreground">
            Running instances of a Workflow — the spec&apos;s core abstraction.
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>Start execution</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Start a new execution</DialogTitle>
            </DialogHeader>
            <StartExecutionForm onStarted={() => setOpen(false)} />
          </DialogContent>
        </Dialog>
      </div>
      <ExecutionTable />
    </div>
  );
}
