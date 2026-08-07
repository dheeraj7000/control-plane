"use client";

import { AnimatePresence, motion } from "framer-motion";
import { usePathname } from "next/navigation";

// The one deliberately light touch of Framer Motion this dashboard
// uses (the spec names it in the planned stack): a short fade/rise on
// route change. Data views (execution timelines, tables) intentionally
// have no per-item enter animation — this is a control plane operators
// read quickly and repeatedly, not a marketing site, so motion is kept
// to "the page changed" feedback only.
export function PageTransition({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  return (
    <AnimatePresence mode="wait">
      <motion.div
        key={pathname}
        initial={{ opacity: 0, y: 4 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.15, ease: "easeOut" }}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
