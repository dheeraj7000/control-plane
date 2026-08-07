"use client";

import { useHealth } from "@/hooks/use-api";
import { useSession } from "@/lib/store";
import { cn } from "@/lib/utils";

export function Header() {
  const { data, isError, isPending } = useHealth();
  const agentName = useSession((s) => s.agentName);
  const apiBaseUrl = useSession((s) => s.apiBaseUrl);

  const status = isPending ? "checking" : isError ? "down" : data?.status === "ok" ? "up" : "unknown";

  return (
    <header className="flex h-14 items-center justify-between border-b border-border px-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span
          className={cn(
            "h-2 w-2 rounded-full",
            status === "up" && "bg-success",
            status === "down" && "bg-destructive",
            status === "checking" && "animate-pulse bg-muted-foreground",
          )}
        />
        <span>
          {apiBaseUrl} — {status === "up" ? "connected" : status === "down" ? "unreachable" : "checking…"}
        </span>
      </div>
      <div className="text-sm">
        {agentName ? (
          <span className="text-muted-foreground">
            Authenticated as <span className="font-medium text-foreground">{agentName}</span>
          </span>
        ) : (
          <span className="text-warning">No credentials — see Settings</span>
        )}
      </div>
    </header>
  );
}
