# Multi-tenancy for SaaS — Plan

Status: **proposed** (CR P3.17). Companion to `docs/sqlite-to-postgres-migration.md` — multi-tenancy is the immediate consumer of that migration and the two have to land together for the SaaS mode to be safe.

## Why now

Today Argus stores everything in a single SQLite file with no tenant column. CR P3.17 calls this out as a HIGH-severity SaaS bottleneck:

> No tenant isolation in DB or config. SaaS mode shares everything.

The blast radius is straightforward: any bug, broken query, or compromised API token can read or write data belonging to any other customer. We can't ship SaaS as-is.

## The model

**One Postgres database, one schema, `tenant_id` column on every row.**

Considered and rejected:

- **One schema per tenant.** Cleaner isolation but onboarding requires running DDL; backups are per-tenant which sounds nice but is operationally heavier. The big win — query-by-default isolation — we get cheaper from RLS below.
- **One Postgres per tenant.** Strongest isolation, totally unworkable at the cost of the smaller customers (one t4g.small per tenant is $7/mo of overhead before they generate $1 of usage). Reserved for "enterprise + on-prem" tier later.

So: shared schema, tenant_id everywhere, RLS as the safety net.

## Tenant identity

A `tenant_id BIGINT` (or UUID; bigint is simpler). One row per customer in a `tenants` table:

```sql
CREATE TABLE tenants (
  id           BIGSERIAL PRIMARY KEY,
  slug         TEXT NOT NULL UNIQUE,        -- used in URLs: argus.app/t/acme/...
  display_name TEXT NOT NULL,
  status       TEXT NOT NULL DEFAULT 'active', -- 'active' | 'suspended' | 'deleted'
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  plan         TEXT NOT NULL DEFAULT 'free' -- 'free' | 'pro' | 'enterprise'
);
```

A `tenant_users` join table maps `auth.users` to `tenants` with a role (`owner` / `admin` / `member` / `viewer`):

```sql
CREATE TABLE tenant_users (
  tenant_id BIGINT  REFERENCES tenants(id) ON DELETE CASCADE,
  user_id   TEXT    REFERENCES users(id)   ON DELETE CASCADE,
  role      TEXT    NOT NULL,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, user_id)
);
```

A user can belong to multiple tenants (an SRE consultant working with two customers); their session carries the **active** tenant_id, switchable through a UI dropdown.

## How tenant_id flows through the codebase

1. **Auth → session.** The auth layer that already issues a session token (passkey or OAuth) adds the chosen `tenant_id` to the session row. Subsequent API requests carry the session cookie; the auth middleware looks up the row and sets `tenant_id` on the request context.

2. **Context propagation.** A new `tenantkey.From(ctx) (tenant_id int64, ok bool)` helper is the single place anything reads tenant_id. Every store call that already takes a `ctx` picks up the value through this helper — no signature changes to existing stores.

3. **Store queries.** Every Postgres-backed store gets a mandatory `WHERE tenant_id = $N` predicate. To make this hard to forget, queries go through a small `db.Tenant(ctx).DB` wrapper that:
   - Looks up tenant_id from ctx.
   - Sets `app.tenant_id` setting on the underlying connection for the duration of the transaction.
   - The pg RLS policy `USING (tenant_id = current_setting('app.tenant_id')::bigint)` enforces isolation even when an engineer forgets the explicit WHERE.

4. **Row-level security as the safety net.** Every tenant-scoped table gets a policy:

   ```sql
   ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;
   CREATE POLICY incidents_tenant_isolation ON incidents
     USING (tenant_id = current_setting('app.tenant_id')::bigint);
   ```

   If the WHERE clause is missing from a query, Postgres still won't return cross-tenant rows. If `app.tenant_id` isn't set, the query returns zero rows — fail-closed.

## Tables in scope

Per `kube/backend/internal/migrations/migrations.go`, the tables that need `tenant_id NOT NULL`:

| Table | Tenant-scoped | Notes |
| --- | --- | --- |
| `incidents` | yes | per-tenant incident store |
| `workflows` | yes | per-tenant runbooks/playbooks |
| `users` | **NO** | a user can belong to multiple tenants; tenant_users carries the link |
| `sessions` | yes | session is for one (user, tenant) pair |
| `oauth_pending` | yes | OAuth state; pending request belongs to a tenant context |
| `agent_profile` | yes | per-tenant AI agent permissions |
| `alert_events` | yes | per-tenant alerts |
| `user_activity` | yes | scoped to tenant for billing/audit |
| `user_profile_mutes` | yes | per-tenant settings |
| `user_suggestion_log` | yes | per-tenant ML feedback |
| `distload_local_runs` | yes | per-tenant load-test history |
| `db_connections` | yes | per-tenant credentials |
| `workspace_connections` | yes | per-tenant OAuth tokens |
| `workspace_tokens` | yes | per-tenant secrets |
| `passkey_credentials` | yes | per-tenant; a user re-registers a passkey per tenant |
| `passkey_sessions` | yes | scoped same as sessions |
| `anomaly_settings` | yes | per-tenant tuning |
| `anomaly_rules` | yes | per-tenant ruleset |
| `tenants` | **NO** | this IS the directory |
| `tenant_users` | yes (via tenant_id col) | scoped, but RLS exempt — needed for the lookup that resolves session→tenant |

`users` is the special case: a single global user can be in multiple tenants. The user table has no `tenant_id`; the join through `tenant_users` is what scopes access. The auth middleware loads tenant_id from the **session**, not the user — that's where multi-tenant membership materializes.

## Migration order

1. **Phase 0 — schema.** Add `tenants` and `tenant_users`. Add `tenant_id` columns to every tenant-scoped table, all `NOT NULL DEFAULT 1` initially. Create one row in `tenants` (id=1, slug='default'); the existing data belongs to it. Drop the DEFAULT once code is fixed.
2. **Phase 1 — context plumbing.** Land `tenantkey` helper + auth-middleware wiring. No store reads use it yet; it just flows.
3. **Phase 2 — query rewrites.** Per-store, in the same dependency order as the Postgres migration plan: anomaly → workspace → agent_profile → … → users/sessions last. Each PR adds the WHERE clause + the RLS policy + a test that two tenants can't see each other's rows.
4. **Phase 3 — onboarding flow.** Sign-up creates a tenant + adds the user as owner; existing users see a "create your first tenant" interstitial.
5. **Phase 4 — tenant switcher UI.** Dropdown in the titlebar, mirrors current context switching.

## Skeleton (this PR will not land code)

The implementation lives behind this plan. The expected starter PR is:

```
kube/backend/internal/tenant/
  tenant.go        # Tenant struct, Repository interface, errors
  context.go       # tenantkey: From(ctx), With(ctx, id)
  middleware.go    # HTTP middleware: session → tenant_id → context
  tenant_test.go
```

— no store changes yet, just the plumbing every other PR will hang off.

## Open questions

1. **Slug vs ID in URLs.** Recommend `slug` for shareable links + `id` internally. Slug is mutable (renames); cookies + RLS use id.
2. **Bypass for support engineers.** Argus internal users may need cross-tenant read for triage. Recommend: a separate `argus_admin` role that bypasses RLS, scoped to a small group, audited via `pg_audit`.
3. **Rate limits per tenant.** P2.12's rate limiter is per-IP. SaaS will want per-tenant too (overlay both). Same interface — just keyed differently. Deferred to follow-up.
4. **Tenant deletion.** Soft-delete (status='deleted') + a backgrounded sweep that hard-deletes after N days. Hard-delete is a `DELETE FROM ... WHERE tenant_id = X` per table — RLS plays nice once `app.tenant_id` is set to X for the sweep job.

## Dependencies

- `docs/sqlite-to-postgres-migration.md` Phase 1 must be in flight for at least one table before this plan's Phase 2 lands — RLS only works on Postgres.
- P2.12 (rate limiting) — the per-IP limiter is in place; per-tenant overlay slots in afterward.

## Refs

- Argus Code Review — P3.17 (multi-tenancy for SaaS)
- Companion plan — `docs/sqlite-to-postgres-migration.md`
