# deployments

Container and deployment artifacts.

- `server.Dockerfile` — multi-stage build for `apps/server`, used by the
  root `docker-compose.yml` for local development.

Kubernetes manifests / Helm charts / Terraform are out of scope for
Phase 1 and will be added alongside Milestone 7 (production hardening)
if and when a target environment is chosen.
