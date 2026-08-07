"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

export function QueryProvider({ children }: { children: React.ReactNode }) {
  // Constructed inside the component (not at module scope) so each
  // request gets its own client under React Server Components /
  // Next.js's app router — a module-level singleton would leak query
  // cache across unrelated requests server-side. Client-side (this is
  // the only place this app actually runs queries, everything here is
  // "use client"), useState still keeps exactly one instance for the
  // component's lifetime.
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            // The dashboard's whole reason for existing is showing
            // live-ish state (execution progress, agent/workflow
            // lists) — a long staleTime would mean stale data by
            // default. 5s balances "reasonably fresh" against not
            // hammering the API on every render.
            staleTime: 5_000,
            retry: 1,
          },
        },
      }),
  );

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
