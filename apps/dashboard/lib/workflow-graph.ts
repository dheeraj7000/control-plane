import type { Step } from "./types";
import type { Edge, Node } from "@xyflow/react";

const COLUMN_WIDTH = 220;
const ROW_HEIGHT = 90;

// layoutSteps positions each step in a column equal to its longest
// path from a root (a step with no DependsOn) — the same notion
// internal/workflow.TopologicalOrder relies on for a valid DAG, just
// computed here purely for visual layout rather than execution order.
// This is a plain longest-path layering, not a general graph-drawing
// algorithm (no edge-crossing minimization) — a workflow's DAG is
// small and shallow enough in practice that a simple layered layout
// reads fine; a real layout engine (dagre, elk) would be the next
// step if that stops being true.
export function layoutSteps(steps: Step[]): { nodes: Node[]; edges: Edge[] } {
  const byId = new Map(steps.map((s) => [s.id, s]));
  const level = new Map<string, number>();

  function levelOf(id: string, seen: Set<string>): number {
    if (level.has(id)) return level.get(id)!;
    if (seen.has(id)) return 0; // guards against a cycle sneaking past backend validation
    seen.add(id);
    const step = byId.get(id);
    const deps = step?.depends_on ?? [];
    const l = deps.length === 0 ? 0 : 1 + Math.max(...deps.map((d) => levelOf(d, seen)));
    level.set(id, l);
    return l;
  }
  steps.forEach((s) => levelOf(s.id, new Set()));

  const countPerLevel = new Map<number, number>();
  const nodes: Node[] = steps.map((step) => {
    const l = level.get(step.id) ?? 0;
    const row = countPerLevel.get(l) ?? 0;
    countPerLevel.set(l, row + 1);
    return {
      id: step.id,
      position: { x: l * COLUMN_WIDTH, y: row * ROW_HEIGHT },
      data: { label: step.name || step.id, type: step.type },
      type: "stepNode",
    };
  });

  const edges: Edge[] = steps.flatMap((step) =>
    (step.depends_on ?? []).map((dep) => ({
      id: `${dep}->${step.id}`,
      source: dep,
      target: step.id,
    })),
  );

  return { nodes, edges };
}
