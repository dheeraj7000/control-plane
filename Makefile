SHELL := /bin/bash
GO ?= go
GOLANGCI_LINT_VERSION := v1.62.2

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
test: ## Run unit tests with race detector
	$(GO) test -race -count=1 ./...

.PHONY: fmt
fmt: ## Format all Go source
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (installs to ./bin if missing)
	@if [ ! -x ./bin/golangci-lint ]; then \
		echo "installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin $(GOLANGCI_LINT_VERSION); \
	fi
	./bin/golangci-lint run ./...

.PHONY: tidy
tidy: ## Sync go.mod/go.sum with imports
	$(GO) mod tidy

.PHONY: check
check: fmt vet test ## fmt + vet + test — run before committing

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
