# PR-8 — viper config swap survey

**Audit recommendation:** swap `lufis-terminal/internal/config` for `spf13/viper`.

**Recommendation:** **reject the swap.** The current config is 130 well-tested lines covering one TOML file plus three env-var overrides for five fields. Viper adds 34 transitive dependencies and ~4 MB to the binary in exchange for features we do not use. The audit appears to have pattern-matched on "Go project loading config" without measuring the actual cost or surveying current functionality.

---

## What the current config does

`internal/config/config.go` — **130 lines, 1 direct dep** (`pelletier/go-toml/v2`, already in the module).

- One on-disk format: TOML at `os.UserConfigDir()/argus-terminal/config.toml`.
- Five fields total: `Shell`, `FontSize`, `Width`, `Height`, `Title`.
- Three env-var overrides (`ARGUS_SHELL`, `ARGUS_TERMINAL_TITLE`, plus the read-only `ArgusContext` bag from `ARGUS_K8S_CONTEXT` / `ARGUS_K8S_NAMESPACE` / `KUBECONFIG`).
- Atomic save (tmp + rename), 0o600 perms.
- 474 lines of test coverage exercising defaults, overrides, error paths, persistence, and the env-only `ArgusContext` semantics.

There is no hot-reload requirement, no profile/overlay system, no remote KV store, no command-line flag binding, and no need for any format other than TOML.

## What viper costs, measured

Same `go build` test, identical `main.go` shape:

| metric | current (pelletier/go-toml/v2) | viper | delta |
| --- | --- | --- | --- |
| direct deps | 1 | 1 | — |
| **transitive deps** | **4** | **38** | **+34** |
| **minimal binary size** | **3.1 MB** | **7.1 MB** | **+4.0 MB** |
| source LOC owned | 130 | 0 | −130 |
| test LOC owned | 474 | unchanged (we still need to test our own usage) | ~0 |

Numbers reproduced via `go build -o app .` of a minimal program that loads a single config file. The viper-side deps include hashicorp/hcl, magiconair/properties, sagikazarmark/locafero, spf13/afero, etc. — every config format viper supports compiles in, whether we use it or not.

## What viper buys us that we don't have

| viper feature | do we need it? |
| --- | --- |
| Multi-format support (JSON, YAML, HCL, INI, properties, dotenv) | no — TOML only, one file |
| Live reload (`WatchConfig`) | no — terminal restart is fine |
| Remote KV stores (etcd, Consul, etc.) | no |
| Cobra/pflag binding | no — no CLI flags today |
| Default value tracking with type coercion | already have it (5 fields, hand-written) |
| Environment-variable prefix binding | already have it (3 env vars, explicit) |
| Sub-config aliasing / overlaying | no |

The only viper feature we'd *plausibly* want is auto-binding env vars by prefix (`viper.SetEnvPrefix("ARGUS")`). For three variables that already have explicit handling, this is not worth the cost.

## What viper takes away

- **34 more deps to vet for CVEs.** Each new minor release of viper pulls more transitive deps along.
- **+4 MB on a desktop binary** that already ships with bundled fonts, freetype, and OpenGL bindings. We're not in a budget where this is invisible.
- **A bigger attack surface.** Viper has, historically, had CVEs around config parsing (e.g. hcl module fuzzer findings). We pay for that exposure even when only loading TOML.
- **Convention lock-in.** Viper's precedence order (explicit `Set` > flag > env > config > default) is fine but rigid. Our current code is explicit — easy to debug, easy to change.

## When this decision should be revisited

If lufis-terminal grows any of these, reopen this survey:

- A second config source (per-domain profiles, workspace-local overrides).
- A second config format we genuinely need (JSON for IDE-generated config, YAML for Helm parity).
- CLI flags with config bindings.
- Remote/dynamic config (push config from Argus backend).

Until then: the current code is doing its job in 130 lines.

## Recommendation

1. **Close PR-8 without code changes.** This document is the deliverable.
2. **Hold the line in code review** — if a future PR proposes viper for an unrelated reason, point at this survey and ask the proposer to justify the dep count.
3. **Open a tiny follow-up** if and when the trigger conditions above are hit. That follow-up should re-measure dep counts at the time, not assume today's numbers.

This is the fourth audit item I've pushed back on (alongside jwt-go-for-credentials, DI-in-main, and gchalk-as-parser). The pattern: tooling recommendations made without measuring cost against what we actually need.
