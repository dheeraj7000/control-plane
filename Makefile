SHELL := /bin/bash
GO ?= go
# v1.62.2 (this repo's original pin) doesn't support the Go 1.26
# toolchain declared in go.mod; v2.12.2 installed via `go install` is
# what's actually verified working (see .github/workflows/ci.yml).
GOLANGCI_LINT_VERSION := v2.12.2

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build all binaries into ./bin
	$(GO) build -o bin/server ./apps/server
	$(GO) build -o bin/control-planectl ./apps/cli

.PHONY: run
run: ## Run the API server locally (expects `make dev-up` infra)
	$(GO) run ./apps/server

.PHONY: test
test: ## Run unit tests with race detector (Postgres integration tests skip themselves)
	$(GO) test -race -count=1 ./...

.PHONY: test-integration
test-integration: ## Run internal/storage's Postgres integration tests (expects `make dev-up` infra)
	TEST_DATABASE_URL="postgres://control_plane:control_plane@localhost:55432/control_plane?sslmode=disable" \
		$(GO) test -race -count=1 ./internal/storage/...

.PHONY: migrate-up
migrate-up: ## Apply pending migrations to DATABASE_URL (or the default local one)
	@$(GO) run ./apps/cli migrate up

.PHONY: fmt
fmt: ## Format all Go source
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (installs to ./bin if missing)
	@if [ ! -x ./bin/golangci-lint ]; then \
		echo "installing golangci-lint $(GOLANGCI_LINT_VERSION) via go install..."; \
		GOBIN=$(CURDIR)/bin $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	./bin/golangci-lint run ./...

.PHONY: tidy
tidy: ## Sync go.mod/go.sum with imports
	$(GO) mod tidy

.PHONY: check
check: fmt vet test ## fmt + vet + test — run before committing

.PHONY: dashboard-dev
dashboard-dev: ## Run the dashboard's Next.js dev server (expects the API server running separately)
	cd apps/dashboard && npm install && npm run dev

.PHONY: dashboard-build
dashboard-build: ## Type-check, lint, and production-build the dashboard
	cd apps/dashboard && npm install && npm run lint && npm run build

.PHONY: dev-up
dev-up: ## Start local infra (Postgres, Redis, NATS) via Docker Compose
	docker compose up -d postgres redis nats --wait

.PHONY: dev-down
dev-down: ## Stop local infra
	docker compose down

.PHONY: dev-logs
dev-logs: ## Tail local infra logs
	docker compose logs -f

.PHONY: docker-up
docker-up: ## Build and start the full stack (server + infra) via Docker Compose
	docker compose up --build

.PHONY: docker-down
docker-down: ## Stop the full stack and remove volumes
	docker compose down -v

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/
