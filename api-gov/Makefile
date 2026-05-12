SHELL := /bin/bash

BACKEND_DIR := backend
AGENTS_DIR  := agents

.PHONY: help dev build run test test-all lint clean deps

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

dev: ## Start all services (Go API + Python agents + DB + Redis)
	docker compose up --build -d
	@echo "  API:    http://localhost:8080"
	@echo "  Agents: http://localhost:8001"
	@echo "  PG:     localhost:5432"
	@echo "  Redis:  localhost:6379"

dev-logs: ## Follow logs
	docker compose logs -f

dev-stop: ## Stop all services
	docker compose down

build: ## Build Go backend
	cd $(BACKEND_DIR) && go build -o build/bin/api-gov ./cmd/server

run: build ## Build and run Go backend
	cd $(BACKEND_DIR) && ./build/bin/api-gov

agent-deps: ## Install Python agent dependencies
	cd $(AGENTS_DIR) && pip install -e ".[dev]"

agent-run: ## Run Python agent server
	cd $(AGENTS_DIR) && python -m src.server

test: test-go test-agent test-middleware ## Run all tests

test-go: ## Run Go tests
	cd $(BACKEND_DIR) && go test ./... -count=1 -v

test-go-race: ## Run Go tests with race detector
	cd $(BACKEND_DIR) && go test ./... -race -count=1

test-agent: ## Run Python agent tests
	cd $(AGENTS_DIR) && python -m pytest tests/ -v -x

test-middleware: ## Run Python middleware tests
	cd middleware && python -m pytest tests/ -v -x

test-all: test-go test-agent test-middleware ## Run everything

lint: lint-go lint-agent ## Run all linters

lint-go: ## Run Go linter
	cd $(BACKEND_DIR) && golangci-lint run ./...

lint-agent: ## Run Python linter
	cd $(AGENTS_DIR) && ruff check src/ tests/

clean: ## Remove build artifacts
	rm -rf $(BACKEND_DIR)/build
