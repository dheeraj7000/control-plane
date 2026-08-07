"use client";

import { useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  Handle,
  Position,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { layoutSteps } from "@/lib/workflow-graph";
import type { Step, StepStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

const STATUS_RING: Record<StepStatus, string> = {
  pending: "border-border",
  running: "border-warning ring-2 ring-warning/40",
  waiting: "border-warning",
  completed: "border-success",
  failed: "border-destructive ring-2 ring-destructive/40",
  skipped: "border-border opacity-60",
};

function StepNode({ data }: NodeProps) {
  const status = data.status as StepStatus | undefined;
  return (
    <div
      className={cn(
        "rounded-md border-2 bg-card px-3 py-2 text-xs shadow-sm",
        status ? STATUS_RING[status] : "border-border",
      )}
    >
      <Handle type="target" position={Position.Left} className="!bg-muted-foreground" />
      <div className="font-medium">{data.label as string}</div>
      <div className="text-muted-foreground">{data.type as string}</div>
      <Handle type="source" position={Position.Right} className="!bg-muted-foreground" />
    </div>
  );
}

const NODE_TYPES = { stepNode: StepNode };

export function WorkflowGraph({
  steps,
  statuses,
}: {
  steps: Step[];
  statuses?: Map<string, StepStatus>;
}) {
  const { nodes, edges } = useMemo(() => {
    const laid = layoutSteps(steps);
    if (statuses) {
      laid.nodes = laid.nodes.map((n) => ({
        ...n,
        data: { ...n.data, status: statuses.get(n.id) ?? "pending" },
      }));
    }
    return laid;
  }, [steps, statuses]);

  return (
    <div className="h-80 w-full rounded-lg border border-border">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        fitView
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
      >
        <Background />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}
