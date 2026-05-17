# PR-9 — plugin system survey (hashicorp/go-plugin vs current)

**Audit recommendation:** evaluate migrating `lufis-terminal/internal/plugin` to `hashicorp/go-plugin`.

**Recommendation:** **reject the migration.** The current in-process system is the right shape for this app. `hashicorp/go-plugin`'s value proposition (process isolation, crash safety, language independence) does not apply here, and its costs (RPC chatter on hot paths, ~10x code surface, serialization-shaped API) are real.

---

## What the current system does

`internal/plugin/manager.go` (131 lines) + `plugin/api/hooks.go` (52 lines) = **183 lines total**, zero external deps beyond stdlib.

Shape:
- `api.Plugin` interface: `Name`, `Version`, `Init(HostAPI)`, `Shutdown`.
- `api.HostAPI`: `Logger`, `RegisterCommand`, `RegisterSidebarPanel`, `RegisterHook`.
- `Manager.Register(plugin)` calls `Init`, indexes by name.
- Plugins compile **into the binary** as Go packages — no dynamic loading, no `plugin.Open`.

In-tree plugins today: `argus-kube`, `db-diag`, `infra-diag`. Each is a Go package that satisfies `api.Plugin`. Registration happens at startup in `cmd/argus-terminal/main.go`.

Hot-path interactions:
- `HookRender` — fired per frame. ~60 Hz potential.
- `Panel.Content func() string` — called every time the sidebar repaints.
- Commands — fired on user keystroke, low frequency.

## What hashicorp/go-plugin is

A library for running plugins as **separate processes** and communicating via gRPC over a Unix socket / named pipe. Used by Terraform providers, Vault auth backends, Packer plugins. The model:

- Host launches plugin binary.
- Plugin advertises a gRPC service descriptor.
- Host calls plugin methods via generated client stubs.
- Plugin crash ≠ host crash.
- Plugins can be written in any language that speaks gRPC.

Cost shape:
- Each plugin call is an RPC: serialize args → write to socket → context switch → deserialize → execute → serialize result → write back → context switch → deserialize.
- Roughly 10–50 µs latency per call on local socket, vs ~ns for a Go function call.
- API must be serializable — `func() string` becomes `GetContent(ctx) (string, error)` and a re-entrancy contract.
- Each plugin = +binary on disk, +process to manage, +schema to version.

## Why the migration is wrong for this app

### 1. Panels are pull-based and called per repaint

`api.Panel.Content func() string` is invoked every render frame to fetch the latest sidebar content. With go-plugin this becomes a gRPC call per frame. Three panels × 60 fps = 180 RPCs/sec just to keep the sidebar drawn. The current code does this with a direct function call.

You can paper over it with caching (plugin pushes content, host caches), but that flips the model from pull to push and breaks the existing API.

### 2. Hooks fire on hot paths

`HookRender`, `HookKeyEvent`, `HookCommandExecuted`. These fire many times per second. RPC chatter on every key event is fine for a server-side workflow tool (Terraform) but not for a terminal that has to feel responsive at 60 fps.

### 3. No multi-tenant plugin problem to solve

go-plugin's killer feature is **isolation**: a third-party Terraform provider that crashes or panics doesn't take Terraform with it. That matters when the plugin author is not the host author.

Here, every plugin is in-tree, written by us, in Go. A plugin panic should crash the terminal — that's a bug we want to know about, not paper over.

### 4. No language-independence requirement

Every existing plugin is Go. There's no proposal for Python/Rust/JS plugins. Paying for language-independence we don't need is the textbook over-engineering anti-pattern.

### 5. Closure-shaped API breaks under serialization

The current host accepts closures:
```go
host.RegisterCommand("pod-list", func(args []string) error { ... })
host.RegisterHook(HookCommandExecuted, func(args interface{}) error { ... })
host.RegisterSidebarPanel("Cluster", api.Panel{Content: func() string { ... }})
```

go-plugin requires schemas. You can't ship a closure over the wire; you ship a request and a response. Every `RegisterX(fn)` call becomes a server-side handler the host RPCs into, which means the *plugin* has to host a gRPC server, not just be a callee. This is a fundamental shape change, not a porting exercise.

## When go-plugin would be right

Reopen this survey if any of these become true:

- We need to load plugins **at runtime** from a directory the user controls (not compiled in).
- We allow **third-party plugins** whose authors we don't control.
- Plugins need to be **untrusted** (sandboxed, with capability limits, with crash isolation).
- We have a **non-Go plugin author** in scope (Python community wants to write a Jupyter integration, etc.).

None of these are on any roadmap I can find.

## Alternative: minor improvements to keep the current system honest

If we want to harden the current model without changing its shape:

1. **Add a `Healthcheck() error`** to `api.Plugin` and call it periodically — surfaces broken plugins without isolating them.
2. **Wrap every host callback in `defer recover()`** so a panicking plugin logs an error instead of taking down the terminal. ~30 lines.
3. **Add a `Stats` method** to `Manager` so the user can see which plugins fired which hooks and how long they took. Useful for diagnosing slowdowns. ~80 lines.

Cumulative cost: ~120 lines of net-new code, no external deps, no API break. This is the right scope for a follow-up PR (call it PR-9a) if any of the symptoms above start hurting.

## Recommendation

1. **Close PR-9 without code changes.** This document is the deliverable.
2. **Do not add `hashicorp/go-plugin`** — it solves problems we don't have at a cost we don't want to pay.
3. **If a future hardening PR is opened**, prefer the panic-recovery + healthcheck additions above to a full RPC migration.

Fifth audit item pushed back on with the same pattern: tooling recommended without matching against actual use case. The audit appears to have pattern-matched on "plugin system in Go" → "hashicorp/go-plugin" without considering that lufis-terminal's plugin system is more like Bubble Tea's components than Terraform's providers.
