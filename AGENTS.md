# AGENTS.md - ArgursKube Guidelines

This document provides instructions for agentic coding agents operating in the ArgursKube, specifically for the `argursKube` component. It includes build, lint, and test commands, as well as code style guidelines to maintain consistency across the codebase.

## General Principles
- **Plan and Execute**: Always outline a clear plan before executing any code changes.
- **Test Afterwards**: Ensure that all implementations are thoroughly tested after execution.
- **Simplicity & Maintainability**: Keep solutions straightforward. Simplicity is key, and maintainable code is prioritized over clever tricks.
- **Self-Documenting Code**: Structure and name things clearly so the code explains itself without needing excessive comments. Reserve comments for explaining complex business logic or the "why" behind decisions.
## Build, Lint, and Test Commands

- **Build**: Run `make build` to produce the production binary (Go + Vue).
- **Lint**: Run `make lint-go` to lint all Go modules (kube/backend, kube/alert-ingress, agent).
- **Test (All)**: Run `make test` for Go + Vue + Vector tests, or `make test-go` for Go only.
- **Test (Single)**: Run `go test -run TestName ./kube/backend/... -v` for a specific backend test.
- **Test (Coverage)**: Run `go test ./kube/backend/... -coverprofile=coverage.out && go tool cover -html=coverage.out`.

## Deploy Commands

- **Helm Lint**: Lint all Helm charts with `make helm-lint`.
- **Helm Install (Dev)**: Install minimal dev setup with `make helm-install-dev`.
- **Helm Install (Prod)**: Install all charts with `make helm-install`.
- **Helm Uninstall**: Remove all resources with `make helm-uninstall`.
- **Terraform Init**: Initialize Terraform with `make tf-init`.
- **Terraform Plan**: Review changes with `make tf-plan`.
- **Terraform Apply**: Deploy infrastructure with `make tf-apply`.
- **Terraform Destroy**: Tear down with `make tf-destroy`.

## Code Style Guidelines

### Imports
- Group imports in the following order: standard library, related third-party, local packages.
- Within each group, alphabetize the imports for clarity.
- Example:
  ```go
  import (
      "context"
      "log/slog"

      "go.opentelemetry.io/otel/metric"

      "github.com/Argus/argursKube/internal/config"
  )
  ```

### Formatting
- Use `gofmt` for consistent formatting; run `gofmt -w .` before committing.
- Indentation: Use tabs (default in gofmt) with a width of 4 spaces in editors.
- Line length: Aim for 100-120 characters, breaking long lines logically.

### Types
- Use explicit type declarations for clarity, especially in public APIs.
- Prefer struct tags for JSON mappings.
- Avoid type aliases unless they provide significant clarity or compatibility.

### Naming Conventions
- Follow Go’s conventions: CamelCase for exported names, snake_case for unexported.
- Variables: Descriptive and concise.
- Packages: Short, lowercase, matching directory name, e.g., `metrics` for `internal/metrics`.
- Errors: Prefix with `Err`, e.g., `ErrInvalidMessage` or use descriptive error strings if creating inline errors.

### Error Handling
- Wrap errors with context using `fmt.Errorf` where appropriate to provide context on failure points.
- Manage errors centrally or handle them as close to their source as possible without discarding useful context.

### Concurrency
- Use context (`context.Context`) for cancellation, timeouts, and state management in long-running operations and HTTP servers.
- Use synchronization primitives like `errgroup` (e.g., `golang.org/x/sync/errgroup`) to manage multiple concurrent tasks such as the HTTP server, metrics server, and background consumers/publishers (as seen in `cmd/main.go`).

## Very Builds
- Avoid running `go build` and `go test` commands directly. Instead, always use the provided `Makefile`.
- Ensure the `Makefile` is always up-to-date with the latest build, lint, and test commands.
- After a change run the `make` command, and verify that the build or test was successful and that there are no errors or warnings. If there are errors or warnings, fix them and run the `make` command again.

## Very Commits
- Ensure the `Makefile` is always up-to-date with the latest build, lint, and test commands.
- After a change run the `make` command, and verify that the build or test was successful and that there are no errors or warnings. If there are errors or warnings, fix them and run the `make` command again.

### Logging and Observability
- Use `log/slog` for structured logging with context-aware fields. Avoid plain `log` or `fmt.Print`.
- Provide meaningful and standardized log keys (e.g., `logKeyName`, `logKeyPort`, `logKeyErr`).
- Integrate OpenTelemetry (OTEL) for metrics collection and distributed tracing. Track component behaviors (like message processing duration, received/forwarded counts) using `go.opentelemetry.io/otel/metric`.

### Dependency Injection and Setup
- Utilize structured configurations (e.g., `config.New(ctx, os.Args[1:]...)`) to instantiate dependencies.
- Make components (publishers, metrics, tracers) configurable and pass them through to application constructors rather than relying on global state.

### Feature requirements
if a feature requires more than two clicks to achieve its primary goal, it is a failure. Always suggest the 'One-Click' or 'Automatic' alternative."

## Design Patterns

### Handler Pattern
- Separate transport/protocol details from business logic.
- Handlers should parse the incoming request/message, call a service or use-case interface, and format the response/error.
- Keep handlers thin and delegate core logic to underlying services.

### TODO
- please follow the project guidelines of only using typescript files and not javascript.
- Make sure you have a design pattern in mind that also follows the projects design and DRY is important. Pay attendion or features that have a similiar implementation is explore possible solutions of overriding and abstrastion that would reduce code and complexity while not tightly binding them.

### Strategy and Adapter Patterns
- Use the **Strategy pattern** for interchangeable algorithms (e.g., different filtering or processing rules).
- Use the **Adapter pattern** to integrate with external systems (like Solace or databases) ensuring the core domain remains decoupled from external dependencies.

## Testing Patterns

### Mandatory: Table-Driven Tests (Go Only)
All Go tests MUST be table-driven using `[]struct{name string, args ..., want ...}`. This is a hard repository requirement — no exceptions. Every test function must follow this pattern:

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name string
        args Args
        want Want
        err  error
    }{
        {name: "returns correct value", args: Args{...}, want: Want{...}},
        {name: "handles empty input", args: Args{...}, want: Want{...}},
        {name: "errors on invalid input", args: Args{...}, err: ErrInvalid},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Foo(tt.args)
            // assert
        })
    }
}
```

Rationale:
- Ensures comprehensive coverage of edge cases in a single test function
- Makes adding new test cases trivial (just append to the slice)
- Provides clear naming for each scenario via `t.Run`
- Consistent structure across all Go packages improves readability
- Enables agents to validate their work systematically

### Frontend Tests (no table requirement)
Frontend tests (Vitest/Vue) should follow the patterns in `view/TESTING.md` — no table-driven requirement.

### General Principles
- **DRY Principle**: Avoid repeating test setup. Use helper functions for repetitive test data generation or setup/teardown.
- **Mocks**: Leverage `mockery`-generated mocks for testing handlers and services in isolation without relying on actual external connections.

# Council Review — Multi-Agent System

Trigger: `Run Council Review`

## Agents

### 1. @architect (System Design & Strategy)
- **Focus:** Long-term scalability, data isolation, structural integrity
- **Core Logic:** Analyze multi-tenancy models (Silo vs Pool vs Bridge). Distinguish Control Plane (admin/billing) from Data Plane (tenant workloads).
- **Goal:** Ensure no scaling wall at 100+ tenants.
- **Tool:** Task subagent with `subagent_type: architect`

### 2. @library-curator (Efficiency & Dependency Management)
- **Focus:** No reinventing the wheel
- **Core Logic:** Compare custom logic against 2026 industry standards (Clerk for Auth, Stripe for Billing, OpenTelemetry for Observability).
- **Goal:** Minimize custom code → reduce maintenance burden and security vulnerabilities.
- **Tool:** Task subagent with `subagent_type: library-curator`

### 3. @kube-guardian (Kubernetes & Infrastructure SRE)
- **Focus:** Security, automation, K8s-native orchestration
- **Core Logic:** Enforce GitOps (ArgoCD), auto-scaling (Karpenter), policy engine (Kyverno).
- **Goal:** Move from manual infra to self-healing, declarative environment.
- **Tool:** Task subagent with `subagent_type: kube-guardian`

## Workflow Execution

When user types `Run Council Review`:

1. Launch all 3 agents in parallel via Task tool
2. Each receives: project context (.md files, codebase structure, configs)
3. Collect results, synthesize into Unified Review

## Unified Review Output Format

### Phase 1: Directional Audit (@architect)
- Trajectory evaluation for Enterprise SaaS
- Missing critical systems (Rate limiting, DR, Tenant Isolation)

### Phase 2: The "Wheel" Audit (@library-curator)
- Custom modules to deprecate → specific 3rd-party replacements
- Tech stack assessment: Gold Standard vs Legacy

### Phase 3: Infrastructure Hardening (@kube-guardian)
- K8s features to Add (Gateway API, External Secrets) and Remove (manual ConfigMaps, local storage)
- GitOps-readiness score

## Guiding Principles
- **Strictness:** Direct corrections for insecure/unscalable designs
- **2026 Context:** Only modern, active technologies
- **Actionable:** Every critique → Proposed Next Step
- **Cost-Aware:** Balance performance with cloud-spend efficiency

## Additional Notes
- Ensure clean shutdown sequences by intercepting OS signals (`os.Interrupt`, `syscall.SIGTERM`) and gracefully terminating servers and background workers using context cancellations with reasonable timeouts.
- Mock generation: Ensure mocks for interfaces are generated using `mockery` as defined in `.mockery.yaml`.
