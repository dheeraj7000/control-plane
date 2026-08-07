#!/usr/bin/env bash
# One-shot local dev setup: copy env defaults, fetch Go deps, start infra.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ ! -f .env ]; then
  cp .env.example .env
  echo "created .env from .env.example"
fi

go mod download
echo "Go dependencies fetched."

make dev-up
echo
echo "Infra is up. Run 'make run' to start the server, or 'make docker-up' to run everything in containers."
