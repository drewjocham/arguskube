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
AGENT_DIR   := agent/python_agents
BUILD_BIN   := $(BACKEND_DIR)/build/bin/$(APP_NAME)

.PHONY: help dev build build-terminal-app build-nopackage run frontend frontend-dev deps deps-go deps-vue \
        test test-go test-vue lint lint-go clean doctor \
        bindings contexts logs \
        agent-deps agent-check agent-test agent-lint agent-format agent-clean

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

build: frontend ## Production build (macOS app bundle + standalone terminal app)
	# Wails runs its own codesign step inside `wails build`. If the .app from
	# a previous run is left in place, the binary may carry stale signatures
	# or extended attributes, and codesign aborts with "resource fork, Finder
	# information, or similar detritus not allowed". Wipe the bin dir first,
	# then re-strip + re-sign after the fresh build for good measure.
	rm -rf $(BACKEND_DIR)/build/bin
	cd $(BACKEND_DIR) && wails build && xattr -cr build/bin/kubewatcher.app && codesign --force --deep --sign - build/bin/kubewatcher.app
	# Build the standalone terminal as a SEPARATE .app bundle with its own
	# CFBundleIdentifier. macOS treats different bundle ids as different
	# applications, which gives the terminal its own Dock icon, Cmd+Tab
	# entry, and Mission Control window — what the user gets when they
	# click "Pop out".
	$(MAKE) build-terminal-app

build-terminal-app: ## Build the standalone Terminal as its own .app bundle inside the main app's Resources/
	@echo "→ Building KubeWatcherTerminal.app …"
	@TERM_APP="$(BACKEND_DIR)/build/bin/kubewatcher.app/Contents/Resources/KubeWatcherTerminal.app"; \
	rm -rf "$$TERM_APP"; \
	mkdir -p "$$TERM_APP/Contents/MacOS" "$$TERM_APP/Contents/Resources"; \
	cd $(BACKEND_DIR) && go build -trimpath -ldflags '-w -s' \
		-o "build/bin/kubewatcher.app/Contents/Resources/KubeWatcherTerminal.app/Contents/MacOS/KubeWatcherTerminal" \
		./cmd/terminal; \
	cp $(BACKEND_DIR)/build/darwin/Info.terminal.plist "$$TERM_APP/Contents/Info.plist"; \
	if [ -f $(BACKEND_DIR)/build/bin/kubewatcher.app/Contents/Resources/iconfile.icns ]; then \
		cp $(BACKEND_DIR)/build/bin/kubewatcher.app/Contents/Resources/iconfile.icns "$$TERM_APP/Contents/Resources/iconfile.icns"; \
	fi; \
	xattr -cr "$$TERM_APP"; \
	codesign --force --deep --sign - "$$TERM_APP"; \
	echo "  ✓ KubeWatcherTerminal.app at $$TERM_APP"

build-nopackage: frontend ## Production binary (no .app bundle)
	cd $(BACKEND_DIR) && wails build -nopackage

run: build-nopackage ## Build then run
	$(BUILD_BIN)

# ── Dependencies ─────────────────────────────────────────────────

deps: deps-go deps-vue agent-deps ## Install all dependencies

deps-go: ## Go module tidy
	cd $(BACKEND_DIR) && go mod tidy

deps-vue: ## npm install for Vue frontend
	cd $(VIEW_DIR) && npm install

# ── Bindings ─────────────────────────────────────────────────────

bindings: ## Regenerate Wails TypeScript bindings
	cd $(BACKEND_DIR) && wails generate module

# ── Testing ──────────────────────────────────────────────────────

test: test-go test-vue agent-test ## Run all tests

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

clean: agent-clean helm-clean ## Remove all build artifacts
	rm -rf $(BACKEND_DIR)/build/bin
	rm -rf $(BACKEND_DIR)/view/dist
	rm -rf $(VIEW_DIR)/dist
	rm -rf $(VIEW_DIR)/node_modules

clean-vue: ## Remove Vue node_modules only
	rm -rf $(VIEW_DIR)/node_modules

# ── Flink ────────────────────────────────────────────────────────

FLINK_DIR := flink

.PHONY: flink-build-gateway flink-build-job flink-test flink-lint

flink-build-gateway: ## Build Flink gateway binary
	cd $(FLINK_DIR)/gateway && CGO_ENABLED=0 go build -ldflags '-s -w' -o ../build/flink-gateway .

flink-build-job: ## Build Flink job Docker image
	docker build -t kubewatcher-flink-job:latest -f $(FLINK_DIR)/Dockerfile $(FLINK_DIR)

flink-test: ## Run Flink gateway tests (none yet — placeholder)
	@echo "  (no tests for Flink gateway yet — add in gateway/ directory)"

flink-lint: ## Lint Flink gateway Go code
	cd $(FLINK_DIR)/gateway && [ -f go.sum ] || go mod tidy
	cd $(FLINK_DIR)/gateway && golangci-lint run ./... 2>/dev/null || echo "  (no golangci-lint config for flink)"
	
# ── Argus Python Agents ──────────────────────────────────────────

AGENT_VENV  := $(AGENT_DIR)/.venv
AGENT_PYTHON := $(AGENT_VENV)/bin/python
AGENT_PIP    := $(AGENT_VENV)/bin/pip

agent-deps: ## Install Argus Python agent dependencies (editable + dev)
	@test -d $(AGENT_VENV) || python3 -m venv $(AGENT_VENV)
	$(AGENT_PIP) install -q -e "$(AGENT_DIR)[dev]"
	@echo "  ✓ Argus agent venv ready at $(AGENT_VENV)"

agent-check: agent-deps ## Validate Argus agents (imports + compile)
	$(AGENT_PYTHON) -c "from argus_agents import *; print('  ✓ All agent imports OK')"
	$(AGENT_PYTHON) -c "from argus_agents.cli import main; print('  ✓ CLI entry point OK')"
	@echo "  ✓ All agent modules compile cleanly"

agent-test: agent-deps ## Run Argus agent tests
	$(AGENT_PYTHON) -m pytest $(AGENT_DIR)/tests -v

agent-lint: ## Lint Argus agents with ruff
	$(AGENT_DIR)/.venv/bin/ruff check $(AGENT_DIR)/src

agent-format: ## Format Argus agents with ruff
	$(AGENT_DIR)/.venv/bin/ruff format $(AGENT_DIR)/src

agent-clean: ## Remove Argus Python agent venv + caches
	rm -rf $(AGENT_VENV)
	find $(AGENT_DIR) -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true

# ── Deploy (Helm + Terraform) ────────────────────────────────────

DEPLOY_DIR   := deploy
HELM_DIR     := $(DEPLOY_DIR)/helm
TERRAFORM_DIR := $(DEPLOY_DIR)/terraform

.PHONY: helm-lint helm-package helm-install helm-uninstall helm-clean \
        tf-init tf-plan tf-apply tf-destroy tf-fmt

helm-deps: ## Update Helm chart dependencies
	helm dependency update $(HELM_DIR)/kubewatcher-monitoring 2>/dev/null || true

helm-lint: helm-deps ## Lint all Helm charts
	@for chart in $(HELM_DIR)/*/; do \
		echo "  Linting $$chart..."; \
		helm lint $$chart; \
	done

helm-package: ## Package all Helm charts
	@mkdir -p $(DEPLOY_DIR)/packages
	@for chart in $(HELM_DIR)/*/; do \
		echo "  Packaging $$chart..."; \
		helm package $$chart --destination $(DEPLOY_DIR)/packages; \
	done

helm-install: ## Install all Helm charts into current k8s context
	@echo "Installing KubeWatcher charts..."
	kubectl create namespace kubewatcher 2>/dev/null || true
	helm upgrade --install kubewatcher-backend $(HELM_DIR)/kubewatcher-backend \
		--namespace kubewatcher --create-namespace
	helm upgrade --install kubewatcher-frontend $(HELM_DIR)/kubewatcher-frontend \
		--namespace kubewatcher

helm-install-dev: ## Install Helm charts with dev overrides
	@echo "Installing KubeWatcher charts (dev mode)..."
	kubectl create namespace kubewatcher 2>/dev/null || true
	helm upgrade --install kubewatcher-backend $(HELM_DIR)/kubewatcher-backend \
		--namespace kubewatcher --create-namespace \
		-f $(HELM_DIR)/kubewatcher-backend/values-dev.yaml
	helm upgrade --install kubewatcher-frontend $(HELM_DIR)/kubewatcher-frontend \
		--namespace kubewatcher \
		-f $(HELM_DIR)/kubewatcher-frontend/values-dev.yaml

helm-uninstall: ## Uninstall Helm charts
	helm uninstall kubewatcher-backend --namespace kubewatcher 2>/dev/null || true
	helm uninstall kubewatcher-frontend --namespace kubewatcher 2>/dev/null || true
	helm uninstall kubewatcher-agent --namespace kubewatcher 2>/dev/null || true
	helm uninstall kubewatcher-alert-ingress --namespace kubewatcher 2>/dev/null || true
	helm uninstall kubewatcher-mcp --namespace kubewatcher 2>/dev/null || true
	helm uninstall kubewatcher-monitoring --namespace kubewatcher 2>/dev/null || true
	kubectl delete namespace kubewatcher 2>/dev/null || true

helm-clean: ## Remove packaged charts
	rm -rf $(DEPLOY_DIR)/packages

tf-init: ## Initialize Terraform
	cd $(TERRAFORM_DIR) && terraform init

tf-plan: ## Terraform plan
	cd $(TERRAFORM_DIR) && terraform plan

tf-apply: ## Terraform apply (deploy infrastructure)
	cd $(TERRAFORM_DIR) && terraform apply

tf-destroy: ## Terraform destroy
	cd $(TERRAFORM_DIR) && terraform destroy

tf-fmt: ## Format Terraform files
	cd $(TERRAFORM_DIR) && terraform fmt -recursive

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
