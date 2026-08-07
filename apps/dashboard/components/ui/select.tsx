import { cn } from "@/lib/utils";
import * as React from "react";

// A plain native <select>, not @radix-ui/react-select — every use of
// this component in the dashboard is a short, flat option list
// (execution state, workflow picker) where the native element's
// built-in keyboard/mobile/accessibility behavior is strictly better
// than reimplementing it, so there's nothing Radix's listbox buys
// here that's worth the extra bundle weight.
export function Select({ className, ...props }: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}
