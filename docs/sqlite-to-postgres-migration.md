# SQLite → Postgres Migration Plan

Status: **proposed** — drafted in response to the Argus Code Review (CR P1.10). Not a commitment to migrate; this document captures the strategy so the work can be sized and scheduled.

## Why migrate

Today Argus stores **everything** in one SQLite file: incidents, workflows, settings, auth (users, sessions, passkeys), alert events, user activity, distload run history, db-connections, workspace OAuth tokens, anomaly rules — 17+ tables across `kube/backend/internal/migrations/migrations.go`.

This is correct for the desktop app. It is **not** correct for the SaaS mode the company is moving toward:

- **One DB per process** — SQLite is a file, not a network service. Two SaaS pods can't share one file safely.
- **No tenant isolation** — every table is global. A SaaS deployment with multiple customers would have to colocate them in one schema, which we can't audit-trail or back up per-tenant.
- **No connection pooling story** — `database/sql` against SQLite caps at ~1 effective writer (file lock). At SaaS load this collapses.
- **Backups are file-level** — point-in-time recovery isn't a thing without a managed Postgres.

CR notes the same: "Single SQLite for everything (HIGH) … No migration path to Postgres."

## Non-goals

- We are **not** dropping SQLite. The desktop app keeps it. The Argus binary continues to choose its store at boot from a flag/env (`ARGUS_DB_DRIVER=sqlite|postgres`).
- We are **not** rewriting query paths to a different ORM. The plan keeps `database/sql` + handwritten queries.
- We are **not** touching `dbagent/` — that's the user-supplied DB analyzer, a separate concern.

## Constraints we have to live with

1. **SQL dialect drift**: SQLite uses `INTEGER PRIMARY KEY`, Postgres uses `BIGSERIAL` / `UUID`. We type-erase the difference at the migration layer, not at the query site.
2. **`AUTOINCREMENT` / `RETURNING`**: SQLite returns the last insert ID via `LastInsertId()`. Postgres prefers `RETURNING id`. We standardize on `RETURNING` and emulate via `LastInsertId` in the SQLite adapter.
3. **`ON CONFLICT(...) DO UPDATE`**: same syntax across both dialects since SQLite 3.24 and Postgres 9.5 — no migration cost.
4. **Datetime storage**: SQLite stores Unix epoch ints today. Postgres prefers `TIMESTAMPTZ`. Keep epoch ints during transition; convert at the application boundary; migrate to TIMESTAMPTZ in a later phase.
5. **JSON columns**: SQLite stores JSON as `TEXT`. Postgres has native `JSONB`. Keep `TEXT` until per-column performance demands it; switching is a `CAST` not a schema rewrite.
6. **WAL vs. WAL mode**: SQLite's WAL is a file mode; Postgres has continuous archiving. No code change needed — different ops concerns.

## Phased plan

### Phase 0 — instrumentation (PR-sized, no behavior change)

- Add a `db.Driver` interface in `internal/db/` that wraps `*sql.DB`. Initial methods: `Begin`, `BeginTx`, `Exec`, `Query`, `QueryRow`. Same shape as `database/sql.DB`, so every existing store changes one line of construction.
- Move dialect-specific SQL strings into a `dialect` parameter on the migration runner (`sqlitedb.Migration` already has the shape; we just add a `postgres` variant of the `up` block per migration).
- Add a `?` ↔ `$N` placeholder rewriter in the driver so 99% of existing queries don't need editing. This is a 30-line helper, well-tested.

### Phase 1 — Postgres alongside SQLite (per-package opt-in)

Order matters — start with stores that have the least cross-table joins:

1. **`anomaly_settings` / `anomaly_rules`** (single-table, settings-shaped). Smallest blast radius.
2. **`workspace_connections` / `workspace_tokens`** (OAuth state — already encrypted via secretref; ciphertext is dialect-agnostic).
3. **`agent_profile`** (single-row settings table).
4. **`db_connections`** (encrypted credentials, similar to workspace tokens).
5. **`incidents` / `alert_events`** (event store, append-mostly — easy to ship double-write for a soak period).
6. **`workflows`** (read-heavy, dedup-prone — needs careful testing of upsert paths).
7. **`distload_local_runs`** (load-test history; large rows, infrequent writes).
8. **`users` / `sessions` / `oauth_pending` / `passkey_credentials` / `passkey_sessions`** (auth — last because auth migrations are the highest-risk path to lock out customers).
9. **`user_activity` / `user_profile_mutes` / `user_suggestion_log`** (analytics/learning — defer; can be re-derived from raw events if needed).

Each step is gated by a CI run-against-Postgres job: spin up Postgres in the workflow, run the touched package's tests, fail loud on dialect-drift bugs. (We don't bring up Postgres for the entire test suite; only the packages that have a Postgres backend yet.)

### Phase 2 — multi-tenancy primitives (concurrent with Phase 1, not after)

Before any SaaS user touches the Postgres-backed stores, we need:

- A `tenant_id BIGINT NOT NULL` column on **every** Postgres table (no exceptions). SQLite tables stay single-tenant by definition; no column there.
- A `tenant_id` propagated through `context.Context` from the API edge. Stores read it via a helper, not by reaching into the global.
- Row-level security policies in Postgres — `CREATE POLICY tenant_isolation ON foo USING (tenant_id = current_setting('app.tenant_id')::bigint)`. Set the setting in `BeginTx` and tear down on commit.
- A migration test that creates two tenants, writes data as each, and confirms tenant A cannot read tenant B's rows even with a direct query.

If RLS turns out to be a bottleneck (it can be on workflows-style read-heavy paths), we fall back to explicit `WHERE tenant_id = $N` in every query. Either way the column is the same.

### Phase 3 — desktop ↔ SaaS replication (not in this PR's scope but called out for completeness)

Eventually a desktop user signs in to SaaS and wants their incidents synced. That's a separate design — likely via the runner's existing SSE path, not a generic CDC pipeline. Out of scope for the migration itself.

## Data migration: existing customer SQLite → SaaS Postgres

When the first SaaS customer onboards from desktop, their SQLite file becomes the source-of-truth seed:

1. **Export** — a one-shot tool reads the SQLite file table-by-table, JSON-encodes each row, and emits an NDJSON stream.
2. **Import** — server-side tool consumes the NDJSON, validates the schema version against `_migrations`, and writes rows into the customer's tenant_id in Postgres.
3. **Reconciliation** — count + checksum per table, fail the import if any row count differs from the export by more than a tolerance (0% for foreign-keyed tables, small tolerance for user_activity).

This tool ships **only after** Phase 1 is complete for every table. Until then, SaaS Postgres starts empty for every new tenant.

## What changes in the codebase

| Layer | Today | After |
| --- | --- | --- |
| Construction | `sqlitedb.Open(dir, logger)` | `db.Open(cfg)` — chooses dialect from `cfg.DB.Driver` |
| Migrations | `[]Migration{...up: "CREATE TABLE..."}` | `[]Migration{...up: map[Dialect]string{...}}` |
| Stores | `*sql.DB` field | `db.Conn` interface field |
| Placeholders | `?` | `?` (rewritten to `$N` by driver under Postgres) |
| `LastInsertId` | direct | wrapped — driver substitutes `RETURNING id` under Postgres |
| `time.Time` storage | Unix epoch int | unchanged for now; TIMESTAMPTZ later |

## Test strategy

- Every new test runs against both dialects via `testdb.NewForBoth(t, ...)` — spin up Postgres via testcontainers-go for the Postgres side, SQLite in-memory for the other.
- The migration runner gets a "round-trip" test per migration: apply up, write canonical data, apply down, re-apply up, verify the data is still readable (idempotency).
- CI gets a new job `backend-postgres` that runs the same test suite with `ARGUS_DB_DRIVER=postgres`. Initially marked `continue-on-error: true` until all packages are dialect-clean.

## Rollout & rollback

- **Per-package feature flag**: `ARGUS_DB_POSTGRES_PACKAGES=anomaly,workspace,...`. The launcher picks the dialect per-store based on this. Lets us flip one table at a time in SaaS without redeploying.
- **Rollback**: per-package flag flip back to SQLite. Data written to Postgres in the meantime is exported via the desktop-export tool and replayed back into a SQLite file.
- **No-op for the desktop app**: the flag is always-empty on desktop. SQLite paths are unchanged.

## Open questions

1. Do we want one Postgres schema per tenant, or one schema with `tenant_id` columns? **Recommendation: tenant_id columns** — fewer DDL changes per onboarding, RLS does the isolation. Schemas-per-tenant is easier to back up individually but operationally heavier.
2. Connection pooling — pgxpool vs. `database/sql` + pgx as a driver? **Recommendation: `database/sql` + pgx** to keep the API uniform with the SQLite path. pgxpool's perf edge isn't worth the dual-API maintenance until profiled.
3. Encryption at rest for credentials — keep the existing `secretref` envelope, or switch to pgcrypto? **Recommendation: keep secretref**; rotating the master key is easier in app code than across Postgres deployments.
4. Migration tool language — Go (consistent with the codebase) or a shell-runner wrapper around `pg_dump|psql`? **Recommendation: Go**, because the schema version check + checksum needs to be cross-dialect aware.

## Effort estimate

- Phase 0 (driver + placeholder rewriter): **1–2 weeks**, 1 engineer.
- Phase 1 (per-table opt-in, 9 tables × roughly 0.5–1 wk each): **6–9 weeks**, can parallelize 2 engineers ~6 wks.
- Phase 2 (multi-tenancy column + RLS): **2–3 weeks** concurrent with late Phase 1.
- Phase 3 (replication): not estimated here — separate project.
- Migration / import tool: **1–2 weeks**, after Phase 1.

**Realistic ship date for "SaaS can run on Postgres":** ~10–12 weeks from start, assuming no surprises in the auth-table migration.

## What this PR does and doesn't

This PR is **the plan document** only. It does not introduce the `db.Driver` interface, does not change `sqlitedb`, does not touch CI. The first implementation step is Phase 0; that lands as a separate PR once this plan is reviewed and the open questions above are decided.

## Refs

- Argus Code Review — P1.10 (plan SQLite → Postgres migration path)
- Argus Code Review — P3.17 (multi-tenancy for SaaS) — depends on Phase 2 of this plan
