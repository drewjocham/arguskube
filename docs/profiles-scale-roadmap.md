# Profiles at 50k users — roadmap

PR #111 ships the profiles feature with backend persistence, sync, quotas, debouncing, structured logging, and 41 tests. This document is the honest list of what's **not** in that PR but matters before scaling past a few thousand active users.

## What's in PR #111

| concern | mitigation | location |
| --- | --- | --- |
| Local correctness | 16 store unit + 10 Wails integration + 15 vitest tests | `internal/profiles/store_test.go`, `api/pkg/app_profiles_test.go`, `view/src/stores/__tests__/profiles.test.ts` |
| Multi-tenant isolation | every Save/Delete validates `user_id` ownership; cross-user attempts return `ErrNotFound` | `internal/profiles/store.go` |
| Hostile-client DB fill | `MaxGroupsPerUser=100`, `MaxVariantsPerGroup=50`, `MaxSnapshotBytes=65536` | `internal/profiles/store.go` |
| Frontend write bursts | 300ms per-target debounce on `syncGroup` / `syncVariant`; delete cancels pending writes | `view/src/stores/profiles.ts` |
| Observability | `slog.Info` on every Save / Delete with hashed user id | `internal/profiles/store.go` |
| Per-IP DoS | existing `ratelimit.PerIP` already wraps the HTTP API | `internal/ratelimit/` |

## What's NOT in PR #111 (priority-ordered)

### 1. SQLite single-writer bottleneck — **P0 for >5k concurrent active users**

`sqlitedb.Open` pins `SetMaxOpenConns(1)`. Every write across the entire backend serializes through one connection: profiles, auth, sessions, workspaces, all of it. At 50k users:

- Reads scale fine — WAL mode supports many readers concurrently.
- Writes contend on a single mutex. The `busy_timeout(5000)` means a request can sit on the bucket for 5 seconds before failing.

**Action:** move the shared store to PostgreSQL behind a `sql.DB` abstraction that the existing packages already use (`*sql.DB` is the parameter type everywhere). The profiles SQL is standard — no migration needed beyond INTEGER→BIGINT for `created_at` columns.

**Estimate:** 1 week (mostly migration script + CI fixture). Track as a separate PR — touches every persistence-layer package.

### 2. Hydrate-burst on session start — **P1 at >10k concurrent sign-ins**

`App.vue` calls `useProfilesStore().hydrate()` once per page load. 50k users hitting the dashboard at typical SaaS rates fires that many `ListProfileGroups` reads per session start.

**Action:** add HTTP-level response caching with a 30-second TTL keyed on session + last-modified header. The store's existing `cachedCallGo` pattern already supports this — just swap the bare `callGo` in `profilesSync.loadFromBackend` for `cachedCallGo` with `FAST_TTL`.

**Estimate:** 30 minutes.

### 3. Load test — **P0 before launch**

I have zero data on how the system behaves at 50k users. The existing `pkg/loadtest` framework can generate the workload.

**Action:** write a load-test scenario that simulates 1k concurrent users each hydrating + making one mutation per minute, and run it against a staging instance for an hour. Measure p99 latency on the four hottest methods (`ListProfileGroups`, `SaveProfileGroup`, `SaveProfileVariant`, `SetActiveProfile`).

**Estimate:** 1 day.

### 4. Account-deletion cascade — **P1 for GDPR**

When a user account is deleted, their profile rows orphan. There's no user-deletion flow today (verified by grepping `DELETE FROM users` — zero matches), so this is theoretical right now, but **GDPR requires it within 30 days of request**.

**Action:** when the user-deletion handler lands, add `DELETE FROM profile_groups WHERE user_id = ?` (and `profile_active`; `profile_variants` rows cascade via the foreign key once enabled). One-line addition to the eventual deletion handler.

### 5. Soft delete + recovery — **P2**

A user who accidentally deletes their main profile group loses all variants under it. No undo.

**Action:** rename the delete operations to "tombstone" (set `deleted_at`), have `ListGroups` filter `WHERE deleted_at IS NULL`, add a background job that hard-deletes after 30 days. Adds two migrations and ~30 lines.

**Estimate:** 2 hours.

### 6. Multi-tab conflict detection — **P3**

User edits a profile in two browser tabs. Both write. Last-write-wins (today) means the older tab's edits silently disappear when they save.

**Action:** add an `updated_at` ETag returned on every Save; client sends it on next Save; backend returns 409 if it doesn't match. Frontend surfaces a "this profile was changed in another tab — reload?" banner.

**Estimate:** 4 hours.

### 7. Snapshot schema validation — **P3**

Backend treats `snapshot_json` as opaque. A frontend bug could write garbage; a future schema migration on the frontend side wouldn't notice old garbage snapshots.

**Action:** add an optional `validateSnapshot` hook that the store calls before persisting. Frontend lands its snapshot schema; store rejects payloads that fail validation.

**Estimate:** 1 day (requires the frontend schema to be locked in first).

## Pre-launch checklist (the 50k-user gate)

- [ ] Item 1 (Postgres migration) merged, OR the existing user count + write rate confirmed below SQLite's single-writer comfort zone (<5 writes/sec sustained).
- [ ] Item 2 (hydrate caching) merged.
- [ ] Item 3 (load test) executed against a staging instance; p99 < 200ms on the four hot methods.
- [ ] Existing per-IP rate limiter verified to still cover the new endpoints (it does — they hit the same `/api/*` handler).
- [ ] Structured-logging output verified in the production log pipeline (Datadog / Loki / wherever).
- [ ] Item 4 (deletion cascade) tracked in the GDPR backlog even if not built yet.

## Items intentionally deferred indefinitely

- **Per-feature metrics export.** Use the structured logs to compute counts in the log pipeline rather than instrument every method. Cheaper to operate.
- **Profile sharing across users.** No product requirement today; would invert the multi-tenant boundary the store is built around.
