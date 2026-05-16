# `internal/k8s/` Sub-package Split — Plan

Status: **proposed** (CR P2.11). 47 Go files in one package; the review flagged it as the hardest-to-maintain area of the codebase. This document picks a target layout and an approach to get there without halting feature work.

## Why this is hard

Look at one of the existing files:

```go
// kube/backend/internal/k8s/topology.go
func (c *Client) BuildTopology(ctx context.Context, namespace string) (*TopologyResult, error) {
    ...
}
```

Every "feature" is a method on the same `*Client` struct. Go does not let methods of a type span packages — you cannot move `BuildTopology` to `topology/` and keep it as `func (c *Client) BuildTopology`. A naive split therefore has to either:

1. **Promote everything to free functions** — `topology.Build(client *k8s.Client, ...)` — which breaks every call site (~30 files outside `internal/k8s/`) and turns Wails bindings inside-out, OR
2. **Split `Client` into smaller typed handles** — `k8s.Client.Topology()` returns a `topology.Client` that holds the underlying clientset — which means defining the seam carefully.

Option 2 is the canonical move for this shape, and it's the one we should take.

## Proposed target layout

```
internal/k8s/
  client.go           # the shared Client (clientset, dynamic, REST mapper, logger)
  impersonate.go      # auth helpers used by every sub-package
  registry.go         # known GVR registry — shared
  helpers_test.go     # shared test fixtures
  doc.go              # package overview

internal/k8s/auth/         # auth_juggler.go
internal/k8s/resources/    # resources*.go (9 files)
internal/k8s/metrics/      # metrics*.go (5 files) + metrics_provider.go
internal/k8s/topology/     # topology*.go
internal/k8s/exec/         # exec.go, ephemeral.go
internal/k8s/logs/         # logs*.go
internal/k8s/ops/          # workload_ops*.go
internal/k8s/diag/         # pod_network_diag*.go, correlator.go, endpoint_analyzer.go
internal/k8s/finops/       # finops*.go, waste_profiler.go
internal/k8s/yaml/         # yamlgen*.go
internal/k8s/gateway/      # gateway*.go
internal/k8s/network/      # resources_network.go, resources_endpointslice.go (subset)
```

Total: ~12 sub-packages + the small shared core. Each sub-package owns the methods that today hang off `*Client` for its area.

## The accessor pattern

`Client` keeps its current construction signature (call sites don't change) and gains lazy sub-package accessors:

```go
// internal/k8s/client.go
type Client struct {
    cs         kubernetes.Interface
    dyn        dynamic.Interface
    mapper     meta.RESTMapper
    logger     *slog.Logger

    // Sub-package facades — lazy-initialized so cold paths don't allocate.
    topologyOnce sync.Once
    topology     *topology.Client
    // ...similarly for resources, metrics, exec, logs, ...
}

func (c *Client) Topology() *topology.Client {
    c.topologyOnce.Do(func() {
        c.topology = topology.New(c.cs, c.dyn, c.mapper, c.logger)
    })
    return c.topology
}
```

Call sites change from `c.BuildTopology(ctx, ns)` to `c.Topology().Build(ctx, ns)`. That's a single-line edit per call site, automatable with `gopls rename` for each method.

The sub-packages depend **only on the shared core** (clientset, mapper, logger). They do **not** depend on each other — when X needs Y, the App layer composes them, not the k8s package.

## Migration order

Order by isolation, not size — fewer methods touching the central `Client` move first so each PR stays reviewable.

1. **`yaml`** — `yamlgen.go` is pure: takes objects, returns strings. Trivial. Use it as the reference implementation for the rest.
2. **`finops`** — entirely cost analytics; doesn't share state with the rest.
3. **`topology`** — self-contained graph builder; one entry point (`BuildTopology`).
4. **`diag`** — pod network diagnostics; depends only on the clientset.
5. **`exec`** — exec / ephemeral; isolated.
6. **`logs`** — log streaming; isolated.
7. **`metrics`** — multi-implementation (metrics-server + Prometheus). Slightly more cross-method coupling; do after `yaml`/`finops` validate the pattern.
8. **`gateway`** — Gateway API migration helper. Self-contained but touches `resources` for object construction. Move after `resources` is in flight.
9. **`resources`** — biggest cluster. Split into two phases:
   - Phase A: workloads + ops + helpers (smaller files first).
   - Phase B: detail / config / storage / network / endpointslice — everything else.
10. **`ops`** — `workload_ops.go`. Move last because it composes resources + topology.

After all the above land, the residual `internal/k8s/` package is just `client.go`, `impersonate.go`, `registry.go`, plus the `doc.go`.

## What each PR looks like

Per sub-package, one PR with this shape:

1. Create `internal/k8s/<area>/`
2. `git mv` the files; rename `package k8s` to `package <area>`
3. Convert method receivers from `(c *Client)` to a fresh `Client` type in the sub-package (it holds the dependencies it needs, no more)
4. Add the lazy accessor on the outer `k8s.Client`
5. Update call sites — usually 5–20 of them, all single-line edits

This is small enough to review and big enough to be useful. Each PR is independent — they can land in any order and don't conflict beyond the shared `client.go`.

## Tests

- Tests move with their files. Package name and import paths update.
- Most existing tests use `*Client` directly; they'll get the same `(c *Client)` → `(c *<area>.Client)` rewrite.
- We may discover tests that reach across what should be sub-package boundaries — those are the seam-violations the review wants us to find. They get inlined or moved out, not preserved as cross-package references.

## What this PR does and doesn't

This PR is the plan document only. The first implementation step is the **`yaml` carve-out** because it's the simplest and validates the pattern. That ships as a separate PR once this plan is reviewed.

## Refs

- Argus Code Review — P2.11 (split internal/k8s/ into sub-packages)
- Companion plan doc — `docs/sqlite-to-postgres-migration.md` (separate concern; both deferred for design review)
