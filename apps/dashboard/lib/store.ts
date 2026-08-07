import { create } from "zustand";
import { persist } from "zustand/middleware";

// Session holds the credentials the dashboard authenticates every API
// call with. There is no control-plane-operator login (internal/auth
// is still unbuilt — see docs/architecture.md's open questions) so
// this is deliberately just "paste an agent bearer token", the same
// credential a programmatic agent would use. The Settings page is
// where a token is obtained (register a new agent) or pasted in
// (reuse an existing one) — see app/settings/page.tsx.
type Session = {
  apiBaseUrl: string;
  agentId: string | null;
  agentName: string | null;
  token: string | null;
  setApiBaseUrl: (url: string) => void;
  setCredentials: (agentId: string, agentName: string, token: string) => void;
  clearCredentials: () => void;
};

export const DEFAULT_API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

export const useSession = create<Session>()(
  persist(
    (set) => ({
      apiBaseUrl: DEFAULT_API_BASE_URL,
      agentId: null,
      agentName: null,
      token: null,
      setApiBaseUrl: (url) => set({ apiBaseUrl: url }),
      setCredentials: (agentId, agentName, token) =>
        set({ agentId, agentName, token }),
      clearCredentials: () => set({ agentId: null, agentName: null, token: null }),
    }),
    {
      // localStorage, not a cookie: this token is sent as an
      // Authorization header (see lib/api.ts), never relied on for
      // ambient/cookie-based auth, so there's no CSRF exposure from
      // persisting it client-side the way there would be for a
      // cookie-based session.
      name: "control-plane-dashboard-session",
    },
  ),
);
