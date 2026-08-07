"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { api } from "@/lib/api";
import { executionWsUrl } from "@/lib/api";
import { useSession } from "@/lib/store";
import type { ExecutionState } from "@/lib/types";

// keys centralizes query-key construction so invalidation (e.g. after
// a mutation, or on a live WebSocket event) can't typo a key that
// silently misses the cache entry it meant to invalidate.
export const keys = {
  health: ["health"] as const,
  agents: ["agents"] as const,
  agent: (id: string) => ["agents", id] as const,
  workflows: ["workflows"] as const,
  workflow: (id: string) => ["workflows", id] as const,
  executions: (filter?: { workflowId?: string; state?: ExecutionState | "" }) =>
    ["executions", filter ?? {}] as const,
  execution: (id: string) => ["executions", id] as const,
  timeline: (id: string) => ["executions", id, "timeline"] as const,
  events: (id: string) => ["executions", id, "events"] as const,
  budget: (id: string) => ["executions", id, "budget"] as const,
};

// A running execution's step-driver (gateway.Service.run) is
// synchronous but happens server-side in a background goroutine —
// there's no push notification for "an execution finished" outside
// the per-execution WebSocket stream, so list/detail views for
// in-progress work poll. 2s is frequent enough to feel live for a
// dashboard without being an aggressive polling loop.
const LIVE_POLL_MS = 2_000;

function isTerminal(state: ExecutionState | undefined): boolean {
  return state === "completed" || state === "failed" || state === "cancelled";
}

export function useHealth() {
  return useQuery({
    queryKey: keys.health,
    queryFn: api.health,
    retry: false,
    refetchInterval: 10_000,
  });
}

export function useAgents() {
  return useQuery({ queryKey: keys.agents, queryFn: api.listAgents });
}

export function useRegisterAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { id: string; name: string; allowedTools: string[] }) =>
      api.registerAgent(vars.id, vars.name, vars.allowedTools),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.agents }),
  });
}

export function useWorkflows() {
  return useQuery({ queryKey: keys.workflows, queryFn: api.listWorkflows });
}

export function useWorkflow(id: string) {
  return useQuery({
    queryKey: keys.workflow(id),
    queryFn: () => api.getWorkflow(id),
    enabled: !!id,
  });
}

export function useRegisterWorkflow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (workflow: unknown) => api.registerWorkflow(workflow),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.workflows }),
  });
}

export function useExecutions(filter?: { workflowId?: string; state?: ExecutionState | "" }) {
  return useQuery({
    queryKey: keys.executions(filter),
    queryFn: () => api.listExecutions(filter),
    refetchInterval: LIVE_POLL_MS,
  });
}

export function useExecution(id: string) {
  return useQuery({
    queryKey: keys.execution(id),
    queryFn: () => api.getExecution(id),
    enabled: !!id,
    refetchInterval: (query) => (isTerminal(query.state.data?.state) ? false : LIVE_POLL_MS),
  });
}

export function useStartExecution() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (workflowId: string) => api.startExecution(workflowId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["executions"] }),
  });
}

export function useTimeline(executionId: string, live: boolean) {
  return useQuery({
    queryKey: keys.timeline(executionId),
    queryFn: () => api.getTimeline(executionId),
    enabled: !!executionId,
    refetchInterval: live ? LIVE_POLL_MS : false,
  });
}

export function useEvents(executionId: string, live: boolean) {
  return useQuery({
    queryKey: keys.events(executionId),
    queryFn: () => api.getEvents(executionId),
    enabled: !!executionId,
    refetchInterval: live ? LIVE_POLL_MS : false,
  });
}

export function useBudget(executionId: string, live: boolean) {
  return useQuery({
    queryKey: keys.budget(executionId),
    queryFn: () => api.getBudget(executionId),
    enabled: !!executionId,
    refetchInterval: live ? LIVE_POLL_MS : false,
  });
}

// useExecutionSocket opens the live WebSocket stream
// (GET .../executions/{id}/ws) and invalidates the timeline/events/
// budget/execution queries whenever a new event arrives, rather than
// hand-merging the pushed event into the query cache. internal/
// gateway's Bus deliberately has no replay for late subscribers (see
// docs/architecture.md), so the socket is purely a "something
// changed, go re-fetch" signal here — GET .../events already remains
// the source of truth this dashboard reads from, exactly the
// "GET history, then the socket for what's next" pattern the backend
// was built around. This trades a small amount of redundant polling
// for not having to reconcile two different representations of the
// same event.
export function useExecutionSocket(executionId: string, enabled: boolean) {
  const qc = useQueryClient();
  const token = useSession((s) => s.token);

  useEffect(() => {
    if (!enabled || !executionId) return;

    const ws = new WebSocket(executionWsUrl(executionId));

    ws.onmessage = () => {
      qc.invalidateQueries({ queryKey: keys.timeline(executionId) });
      qc.invalidateQueries({ queryKey: keys.events(executionId) });
      qc.invalidateQueries({ queryKey: keys.budget(executionId) });
      qc.invalidateQueries({ queryKey: keys.execution(executionId) });
    };
    ws.onerror = () => {
      // Polling above already keeps the UI correct without the
      // socket (see LIVE_POLL_MS) — a failed connection (stale token,
      // network hiccup, server restart) is a silent degrade to
      // polling-only, not a broken page.
    };

    return () => {
      ws.close();
    };
    // token is a real dependency here (see executionWsUrl) — a token
    // change (e.g. switching agents in Settings) must reconnect with
    // the new one, not keep streaming under the old identity.
  }, [executionId, enabled, qc, token]);
}
