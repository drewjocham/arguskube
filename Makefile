# ──────────────────────────────────────────────────────────────────
# KubeWatcher — Makefile
# ──────────────────────────────────────────────────────────────────
#
#   make dev          — hot-reload desktop app (Go + Vue)
#   make build        — production binary
#   make run          — build then run
#   make frontend     — rebuild Vue into backend/view/dist
#   make test         — all Go + Vue tests
#   make lint         — Go linter + Vue type-check
#   make clean        — nuke build artifacts
#   make deps         — install everything
#   make doctor       — check all required tools
#
# ──────────────────────────────────────────────────────────────────

SHELL := /bin/bash

# Kubeconfig — edit or override:  make dev KUBECONFIG=...
KUBECONFIG ?= $(HOME)/.kube/config_dir/local-config:$(HOME)/.kube/config_dir/k3s-lab-config
export KUBECONFIG

APP_NAME    := kubewatcher
BACKEND_DIR := backend
VIEW_DIR    := view
BUILD_BIN   := $(BACKEND_DIR)/build/bin/$(APP_NAME)

.PHONY: help dev build run frontend frontend-dev deps deps-go deps-vue \
        test test-go test-vue lint lint-go clean doctor \
        bindings contexts logs

# ── Default ──────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ── Development ──────────────────────────────────────────────────

dev: ## Hot-reload desktop app (Go + Vue)
	cd $(BACKEND_DIR) && wails dev

frontend: ## Build Vue into backend/view/dist
	cd $(VIEW_DIR) && npm run build

frontend-dev: ## Start Vite dev server standalone (for debugging)
	cd $(VIEW_DIR) && npm run dev

# ── Build ────────────────────────────────────────────────────────

build: frontend ## Production build (macOS app bundle)
	cd $(BACKEND_DIR) && wails build && xattr -cr build/bin/kubewatcher.app && codesign --force --deep --sign - build/bin/kubewatcher.app

build-nopackage: frontend ## Production binary (no .app bundle)
	cd $(BACKEND_DIR) && wails build -nopackage

run: build-nopackage ## Build then run
	$(BUILD_BIN)

# ── Dependencies ─────────────────────────────────────────────────

deps: deps-go deps-vue ## Install all dependencies

deps-go: ## Go module tidy
	cd $(BACKEND_DIR) && go mod tidy

deps-vue: ## npm install for Vue frontend
	cd $(VIEW_DIR) && npm install

# ── Bindings ─────────────────────────────────────────────────────

bindings: ## Regenerate Wails TypeScript bindings
	cd $(BACKEND_DIR) && wails generate module

# ── Testing ──────────────────────────────────────────────────────

test: test-go test-vue ## Run all tests

test-go: ## Run Go tests
	cd $(BACKEND_DIR) && go test ./... -count=1

test-vue: ## Run Vue/Vitest tests
	cd $(VIEW_DIR) && npm run test:run

test-vue-ui: ## Vitest browser UI
	cd $(VIEW_DIR) && npm run test:ui

# ── Linting ──────────────────────────────────────────────────────

lint: lint-go ## Run all linters

lint-go: ## Run golangci-lint
	cd $(BACKEND_DIR) && golangci-lint run ./...

# ── Kubernetes Helpers ───────────────────────────────────────────

contexts: ## List available kubeconfig contexts
	kubectl config get-contexts

logs: ## Tail logs from all pods (requires stern)
	stern -A ".*" --tail 20

pods: ## List all pods across namespaces
	kubectl get pods -A --sort-by=.metadata.namespace

# ── Cleanup ──────────────────────────────────────────────────────

clean: ## Remove all build artifacts
	rm -rf $(BACKEND_DIR)/build/bin
	rm -rf $(BACKEND_DIR)/view/dist
	rm -rf $(VIEW_DIR)/dist
	rm -rf $(VIEW_DIR)/node_modules

clean-vue: ## Remove Vue node_modules only
	rm -rf $(VIEW_DIR)/node_modules

# ── Doctor ───────────────────────────────────────────────────────

doctor: ## Check all required tools are installed
	@echo "Checking tools..."
	@printf "  %-14s" "go" && (go version 2>/dev/null | head -1 || echo "MISSING — install from https://go.dev")
	@printf "  %-14s" "wails" && (wails version 2>/dev/null | head -1 || echo "MISSING — go install github.com/wailsapp/wails/v2/cmd/wails@latest")
	@printf "  %-14s" "node" && (node --version 2>/dev/null || echo "MISSING — install from https://nodejs.org")
	@printf "  %-14s" "npm" && (npm --version 2>/dev/null || echo "MISSING")
	@printf "  %-14s" "kubectl" && (kubectl version --client --short 2>/dev/null || kubectl version --client 2>/dev/null | head -1 || echo "MISSING")
	@printf "  %-14s" "KUBECONFIG" && echo "$(KUBECONFIG)"
	@echo ""
	@echo "Contexts:"
	@kubectl config get-contexts 2>/dev/null || echo "  (no kubeconfig found)"
