import { useSession } from "./store";
import type {
  Agent,
  Budget,
  ControlPlaneEvent,
  Execution,
  ExecutionState,
  RegisterAgentResponse,
  TimelineEntry,
  Workflow,
} from "./types";

// ApiClientError carries the HTTP status alongside the server's error
// message (every handler in internal/gateway/http.go responds with
// {"error": "..."} on failure — writeServiceError/writeError) so
// callers can distinguish, e.g., a 409 conflict from a 401.
export class ApiClientError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
  }
}

async function request<T>(
  path: string,
  options: { method?: string; body?: unknown; auth?: boolean } = {},
): Promise<T> {
  const { apiBaseUrl, token } = useSession.getState();
  const { method = "GET", body, auth = true } = options;

  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth && token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${apiBaseUrl}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  // 204/No body responses aren't used by this API today, but guard
  // anyway rather than assuming every response has a JSON body.
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;

  if (!res.ok) {
    const message =
      (data && typeof data === "object" && "error" in data && String(data.error)) ||
      res.statusText ||
      `request failed with status ${res.status}`;
    throw new ApiClientError(res.status, message);
  }
  return data as T;
}

// executionWsUrl converts the configured HTTP(S) API base URL into the
// ws(s):// equivalent for the live event stream endpoint. The token is
// passed as a query parameter rather than a header — browsers can't
// set arbitrary headers during a WebSocket handshake — which is why
// this one endpoint has its own AuthMiddlewareWS on the Go side (see
// internal/gateway/auth.go) instead of the header-only scheme every
// other route uses.
export function executionWsUrl(executionId: string): string {
  const { apiBaseUrl, token } = useSession.getState();
  const wsBase = apiBaseUrl.replace(/^http/, "ws");
  const qs = token ? `?token=${encodeURIComponent(token)}` : "";
  return `${wsBase}/executions/${executionId}/ws${qs}`;
}

export const api = {
  health: () => request<{ status: string }>("/healthz", { auth: false }),

  registerAgent: (id: string, name: string, allowedTools: string[]) =>
    request<RegisterAgentResponse>("/agents", {
      method: "POST",
      body: { id, name, allowed_tools: allowedTools },
      auth: false, // matches the backend: agent registration is unauthenticated
    }),
  getAgent: (id: string) => request<Agent>(`/agents/${encodeURIComponent(id)}`),
  listAgents: () => request<Agent[]>("/agents"),

  registerWorkflow: (workflow: unknown) =>
    request<Workflow>("/workflows", { method: "POST", body: workflow }),
  getWorkflow: (id: string) => request<Workflow>(`/workflows/${encodeURIComponent(id)}`),
  listWorkflows: () => request<Workflow[]>("/workflows"),

  startExecution: (workflowId: string) =>
    request<Execution>("/executions", { method: "POST", body: { workflow_id: workflowId } }),
  getExecution: (id: string) => request<Execution>(`/executions/${encodeURIComponent(id)}`),
  listExecutions: (filter?: { workflowId?: string; state?: ExecutionState | "" }) => {
    const params = new URLSearchParams();
    if (filter?.workflowId) params.set("workflow_id", filter.workflowId);
    if (filter?.state) params.set("state", filter.state);
    const qs = params.toString();
    return request<Execution[]>(`/executions${qs ? `?${qs}` : ""}`);
  },
  getTimeline: (id: string) => request<TimelineEntry[]>(`/executions/${encodeURIComponent(id)}/timeline`),
  getEvents: (id: string) => request<ControlPlaneEvent[]>(`/executions/${encodeURIComponent(id)}/events`),
  getBudget: (id: string) => request<Budget>(`/executions/${encodeURIComponent(id)}/budget`),
};
