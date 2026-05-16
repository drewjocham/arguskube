APP_NAME    := argus-terminal
CMD_DIR     := ./cmd/$(APP_NAME)
BUILD_DIR   := build
LDFLAGS     := -w -s

.DEFAULT_GOAL := help

.PHONY: help build run test test-v lint vet check fmt tidy clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "$(cyan)%-20s$(reset) %s\n", $$1, $$2}'
	@printf "\n  $(dim)Legend: $(cyan)command$(reset) $(dim)description$(reset)\n"

build: ## Build the terminal binary
	@echo "→ Building $(APP_NAME)…"
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "✓ $(BUILD_DIR)/$(APP_NAME)"

run: build ## Build and run
	@echo "→ Running $(APP_NAME)…"
	@./$(BUILD_DIR)/$(APP_NAME)

test: ## Run all tests
	go test ./... -count=1

test-v: ## Run all tests with verbose output
	go test ./... -count=1 -v

lint: ## Run golangci-lint
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

check: fmt vet lint test ## Run all quality gates

fmt: ## Format all Go code
	go fmt ./...

tidy: ## Tidy go modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

# Colors for help output
cyan := $(shell tput setaf 6 2>/dev/null || echo "")
dim  := $(shell tput dim 2>/dev/null || echo "")
reset := $(shell tput sgr0 2>/dev/null || echo "")
