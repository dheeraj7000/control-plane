import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Produces a minimal, self-contained .next/standalone build (just
  // the traced production dependencies, not the full node_modules) —
  // what deployments/dashboard.Dockerfile's runtime stage copies.
  output: "standalone",
};

export default nextConfig;
