# AGENTS.md - ArgursKube Guidelines

This document provides instructions for agentic coding agents operating in the ArgursKube, specifically for the `argursKube` component. It includes build, lint, and test commands, as well as code style guidelines to maintain consistency across the codebase.

## General Principles
- **Plan and Execute**: Always outline a clear plan before executing any code changes.
- **Test Afterwards**: Ensure that all implementations are thoroughly tested after execution.
- **Simplicity & Maintainability**: Keep solutions straightforward. Simplicity is key, and maintainable code is prioritized over clever tricks.
- **Self-Documenting Code**: Structure and name things clearly so the code explains itself without needing excessive comments. Reserve comments for explaining complex business logic or the "why" behind decisions.
## Build, Lint, and Test Commands

- **Build**: Compile the project with `go build -o argursKube ./cmd/main.go`.
- **Lint**: Run static code analysis using `golangci-lint run ./...` to ensure code quality.
- **Test (All)**: Execute all tests with `go test ./... -v` for full coverage.
- **Test (Single)**: Run a specific test with `go test -run TestName ./path/to/package -v`.
- **Test (Coverage)**: Generate test coverage with `go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out`.

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

### Logging and Observability
- Use `log/slog` for structured logging with context-aware fields. Avoid plain `log` or `fmt.Print`.
- Provide meaningful and standardized log keys (e.g., `logKeyName`, `logKeyPort`, `logKeyErr`).
- Integrate OpenTelemetry (OTEL) for metrics collection and distributed tracing. Track component behaviors (like message processing duration, received/forwarded counts) using `go.opentelemetry.io/otel/metric`.

### Dependency Injection and Setup
- Utilize structured configurations (e.g., `config.New(ctx, os.Args[1:]...)`) to instantiate dependencies.
- Make components (publishers, metrics, tracers) configurable and pass them through to application constructors rather than relying on global state.

## Design Patterns

### Handler Pattern
- Separate transport/protocol details from business logic.
- Handlers should parse the incoming request/message, call a service or use-case interface, and format the response/error.
- Keep handlers thin and delegate core logic to underlying services.

### Strategy and Adapter Patterns
- Use the **Strategy pattern** for interchangeable algorithms (e.g., different filtering or processing rules).
- Use the **Adapter pattern** to integrate with external systems (like Solace or databases) ensuring the core domain remains decoupled from external dependencies.

## Testing Patterns

- **Table-Driven Tests**: Use table-driven testing (`[]struct{name string, args ..., want ...}`) extensively to systematically cover various input scenarios.
- **DRY Principle**: Avoid repeating test setup. Use helper functions for repetitive test data generation or setup/teardown.
- **Mocks**: Leverage `mockery`-generated mocks for testing handlers and services in isolation without relying on actual external connections.

## Additional Notes
- Ensure clean shutdown sequences by intercepting OS signals (`os.Interrupt`, `syscall.SIGTERM`) and gracefully terminating servers and background workers using context cancellations with reasonable timeouts.
- Mock generation: Ensure mocks for interfaces are generated using `mockery` as defined in `.mockery.yaml`.
