# Plan: OAuth-Driven Integration — Google Identity, Google Workspace, iCloud

**Date:** 2026-05-18
**Goal:** Users sign in with Google OAuth, then connect Google Workspace and iCloud to access docs, notes, calendar, and tasks/reminders natively. All three pillars share the connection model already established by the workspace integration framework.

---

## Current State Assessment

### Two separate OAuth systems in the codebase

| System | Purpose | Status |
|---|---|---|
| **Auth OAuth** (`oauthproviders/`) | "Sign in with Google" — identity/authentication into Argus | DONE (needs audit) |
| **Workspace OAuth** (`workspace/`) | Connect Google/iCloud — access user data (docs, calendar, etc.) | PARTIAL |

### What exists (strong foundation)

| Component | Path | Status |
|---|---|---|
| Auth OAuth Provider framework | `kube/backend/internal/oauthproviders/` | DONE |
| Auth OAuth Wails handlers | `kube/backend/api/pkg/app_oauth.go` | DONE |
| Google sign-in (identity) | `kube/backend/api/pkg/auth_handler.go`, `docs/auth-google-signin.md` | DONE |
| Apple sign-in (identity) | `kube/backend/api/pkg/auth_handler.go`, `kube/docs/auth-apple-signin.md` | DONE |
| Workspace Manager (OAuth orchestration) | `kube/backend/internal/workspace/manager.go` | DONE |
| Provider + Refresher interfaces | `kube/backend/internal/workspace/types.go` | DONE |
| Token storage (AES-256-GCM) | `kube/backend/internal/workspace/storage.go` | DONE |
| Background refresh worker | `kube/backend/internal/workspace/refresh_worker.go` | DONE |
| Google Workspace OAuth Provider (unified grant) | `kube/backend/internal/workspace/google.go` | DONE |
| Google Docs adapter | `kube/backend/internal/workspace/gdocs.go` | DONE |
| Google Sheets adapter | `kube/backend/internal/workspace/gsheets.go` | DONE |
| Google Tasks adapter | `kube/backend/internal/workspace/gtasks.go` | DONE |
| Google Chat adapter | `kube/backend/internal/workspace/gchat.go` | DONE |
| Wails bindings (workspace) | `kube/backend/api/pkg/app_workspace*.go` | DONE |
| HTTP callback handler (workspace) | `kube/backend/api/pkg/workspace_handler.go` | DONE |
| OAuth Login button (frontend) | `kube/view/src/components/auth/OAuthLoginButton.vue` | DONE |
| Hermes google-workspace skill | `~/.hermes/skills/productivity/google-workspace/` | DONE (needs credential setup) |

### What's missing (the gaps)

| Capability | Google | iCloud |
|---|---|---|
| Identity OAuth credential setup | EXISTS in code, needs GCP console config + env vars | N/A (Apple sign-in is separate) |
| Calendar | MISSING adapter | MISSING (CalDAV or CloudKit) |
| Notes | Via Docs (exists) | MISSING (apple-notes skill exists as CLI) |
| Tasks/Reminders | EXISTS (gtasks.go) | MISSING (apple-reminders skill exists as CLI) |
| Files/Drive | Via Drive scope (not yet wired) | MISSING (iCloud Drive) |
| OAuth Provider (workspace) | EXISTS (google.go) | MISSING (needs app-specific password flow) |

### Scope gaps in existing Google Workspace OAuth

Current `googleScopes` in `google.go` line 34-46:
```
documents + spreadsheets + tasks + chat.spaces.readonly + chat.messages.create + userinfo.email + userinfo.profile
```
**Missing:** Calendar scope (`https://www.googleapis.com/auth/calendar`).

---

## Proposed Approach

Two tracks run in parallel because they use different OAuth clients in Google Cloud Console:

### Track A: Google Identity OAuth (authentication)
- One GCP OAuth client ("Desktop app" type) for "Sign in with Google"
- Loopback callback to `http://127.0.0.1:8080/auth/google/callback`
- Already implemented — needs credential setup + end-to-end verification

### Track B: Workspace OAuth (data access)  
- Separate GCP OAuth client for workspace integrations
- Follow the **established Provider + Adapter pattern**
- Each new integration (calendar, icloud) gets a Provider, adapter, Wails bindings, and frontend UI

### Architecture

```
TRACK A — Sign in with Google (identity)
  User clicks "Continue with Google" on login screen
    → App.StartOAuthFlow("google")                     [Wails]
    → oauthproviders.Manager.Start()
    → Browser opens accounts.google.com consent
    → Redirect to http://127.0.0.1:8080/auth/google/callback
    → Backend exchanges code, creates session
    → UI polls /auth/oauth/poll?state=... → logged in

TRACK B — Connect Google Workspace (data access)
  User clicks "Connect Google" in Workspace panel
    → App.ConnectWorkspace("google")                   [Wails]
    → workspace.Manager.Start(ctx, userID, ServiceGoogle, redirectURL)
    → Browser opens accounts.google.com consent (Docs+Sheets+Tasks+Calendar)
    → Redirect to /workspace/oauth/callback?state=...&code=...
    → Manager.Complete() exchanges code, stores encrypted tokens
    → UI receives postMessage → connection appears in sidebar
```

---

## Step-by-Step Plan

### Phase 0: Google Identity OAuth — Credential Setup & Verification

**Goal:** Ensure "Sign in with Google" works end-to-end. The code is done; this is about GCP console configuration, env vars, and testing.

**Prerequisites (one-time, in Google Cloud Console):**
1. Create or select a project at https://console.cloud.google.com
2. Create OAuth 2.0 Client ID → Application type: "Desktop app"
3. Add authorized redirect URI: `http://127.0.0.1:8080/auth/google/callback`
4. Download the client secret JSON file

**Files to verify (read-only audit):**
- `kube/backend/internal/oauthproviders/` — provider registry, preset configs
- `kube/backend/api/pkg/app_oauth.go` — Wails handlers: `ListOAuthProviders`, `StartOAuthFlow`, `CompleteOAuthFlow`, `PollOAuthFlow`
- `kube/backend/api/pkg/auth_handler.go` — HTTP callback handler at `/auth/google/callback`
- `docs/auth-google-signin.md` — documented setup instructions

**Steps:**
1. Set env vars: `ARGUS_GOOGLE_CLIENT_ID`, `ARGUS_GOOGLE_CLIENT_SECRET`, `ARGUS_AUTH_BASE_URL=http://127.0.0.1:8080`
2. Verify `auth_handler.go` registers the `/auth/google/callback` route
3. Verify the oauthproviders manager has Google preset configured at boot
4. End-to-end test: launch app, click "Continue with Google", complete sign-in
5. Verify session persistence (refresh, close/reopen app — still signed in)

**Also verify Apple Sign in with Apple** (already implemented, needs same audit):
- Verify env vars: `ARGUS_APPLE_SERVICES_ID`, `ARGUS_APPLE_TEAM_ID`, `ARGUS_APPLE_KEY_ID`, `ARGUS_APPLE_PRIVATE_KEY_FILE`
- Note: Apple requires HTTPS + public host for callback (ngrok/cloudflared tunnel for local dev)

**Validation:** Manual end-to-end test on macOS desktop build

### Phase 1: Google Calendar Adapter (lowest effort, highest impact)

**Files to create:**
- `kube/backend/internal/workspace/gcal.go` — Calendar adapter
- `kube/backend/internal/workspace/gcal_test.go` — table-driven tests
- `kube/backend/api/pkg/app_workspace_google_cal.go` — Wails bindings
- `kube/backend/api/pkg/app_workspace_google_cal_test.go` — tests

**Files to modify:**
- `kube/backend/internal/workspace/google.go` — add calendar scope to `googleScopes`
- `kube/backend/internal/workspace/types.go` — add `ServiceGCal Service = "gcal"` constant + Calendarer interface
- `kube/backend/api/pkg/app_workspace_google.go` — wire gCal adapter singleton

**Steps:**
1. Add `ServiceGCal` to `types.go` and `supportedServices` map
2. Define `Calendarer` interface: `ListEvents(...)`, `CreateEvent(...)`, `UpdateEvent(...)`, `DeleteEvent(...)`
3. Implement `GCalAdapter` in `gcal.go` — hand-rolled HTTP against Google Calendar v3 API (follows gdocs/gsheets pattern, no SDK dependency)
4. Add `https://www.googleapis.com/auth/calendar` to `googleScopes` in `google.go`
5. Add Wails methods: `ListGoogleCalendarEvents`, `CreateGoogleCalendarEvent`, `UpdateGoogleCalendarEvent`, `DeleteGoogleCalendarEvent`
6. Write table-driven tests with mock HTTP server (following `google_test.go` pattern)

**Google scope expansion note:** Existing Google connections won't have the Calendar grant. The `googleScopes` comment already documents this UX path (user must disconnect + reconnect). No code changes needed for re-auth.

**Validation:** `make test-go` in `kube/backend`

### Phase 2: iCloud Integration (platform-native bridge)

**Files to create:**
- `kube/backend/internal/workspace/icloud.go` — iCloud Provider
- `kube/backend/internal/workspace/icloud_test.go` — tests
- `kube/backend/api/pkg/app_workspace_icloud.go` — Wails bindings

**Files to modify:**
- `kube/backend/internal/workspace/types.go` — add `ServiceICloud Service = "icloud"` + sub-services for calendar/notes/reminders

**Architecture decision:** iCloud doesn't offer a standard OAuth flow for third-party apps. Instead, use Apple's **app-specific password** mechanism. The user generates a password at `appleid.apple.com`, and we store it encrypted (same AES-256-GCM as workspace tokens).

**Provider implementation:**
- `ICloudProvider.Start()` — returns a guide URL (`appleid.apple.com`) and prompts the user to enter their app-specific password
- `ICloudProvider.Complete()` — validates the password by making a test request to an iCloud endpoint (CalDAV `PROPFIND`)
- Token storage: the app-specific password is stored as the "access token" (encrypted), with no refresh token or expiry (iCloud app passwords don't expire)

**Capability adapters:**

| Capability | Implementation | Backend |
|---|---|---|
| Calendar | CalDAV protocol (RFC 4791) | iCloud CalDAV server (`caldav.icloud.com`) |
| Notes | Bridge to `memo` CLI (apple-notes skill) | macOS AppleScript/EventKit via CLI |
| Reminders | Bridge to `remindctl` CLI (apple-reminders skill) | macOS AppleScript/EventKit via CLI |
| iCloud Drive | CloudKit Web Services or WebDAV | `icloud.com` API |

**For macOS desktop (primary target):**
- Desktop app runs on macOS — shell out to `memo` and `remindctl`
- CalDAV for calendar: `curl`-based PROPFIND/REPORT against `https://caldav.icloud.com/` with the app-specific password

**Wails methods:**
- `ConnectICloud(sessionToken, appleID, appPassword string) (Connection, error)` — validate + store
- `ListICloudEvents(sessionToken, connectionID, range) ([]Event, error)` — CalDAV query
- `ListICloudNotes(sessionToken) ([]Note, error)` — delegate to `memo` CLI
- `ListICloudReminders(sessionToken) ([]Reminder, error)` — delegate to `remindctl` CLI

**Validation:** Manual testing on macOS (requires iCloud account + app-specific password)

### Phase 3: Hermes google-workspace Skill — OAuth Credential Setup

**Goal:** The `google-workspace` Hermes skill needs its own Google OAuth credentials so the Hermes agent can interact with Gmail, Calendar, Drive, Docs, Sheets on behalf of the user from the CLI/chat. This is separate from the Argus app's OAuth — it's for the Hermes agent runtime.

**Current status of the skill:**

The `google-workspace` skill at `~/.hermes/skills/productivity/google-workspace/` reports:
```
Missing credential files: google_token.json, google_client_secret.json
Readiness: setup_needed
```

**Steps (non-interactive, following the skill's setup instructions):**
1. **Check current state:**
   ```bash
   python ~/.hermes/skills/productivity/google-workspace/scripts/setup.py --check
   ```
2. **GCP console setup** (user must do this):
   - Create a **separate** OAuth 2.0 Client ID in Google Cloud Console (Desktop app type)
   - Enable APIs: Gmail, Calendar, Drive, Sheets, Docs, People
   - Download the `client_secret_*.json` file
3. **Register the client secret:**
   ```bash
   python ~/.hermes/skills/productivity/google-workspace/scripts/setup.py --client-secret /path/to/client_secret.json
   ```
4. **Generate OAuth URL and have user authorize:**
   ```bash
   python ~/.hermes/skills/productivity/google-workspace/scripts/setup.py --auth-url --services all --format json
   ```
   → Send the `auth_url` to the user, they paste back the redirect URL with code
5. **Exchange the code:**
   ```bash
   python ~/.hermes/skills/productivity/google-workspace/scripts/setup.py --auth-code "THE_URL_OR_CODE"
   ```
6. **Verify:**
   ```bash
   python ~/.hermes/skills/productivity/google-workspace/scripts/setup.py --check
   ```
   → Should print `AUTHENTICATED`

**Note:** This GCP OAuth client is separate from the two used by Argus (Track A identity + Track B workspace). Three different OAuth clients total, but they share the same Google APIs and can live in the same GCP project.

**Validation:** `setup.py --check` prints `AUTHENTICATED`

### Phase 4: Frontend Connection UI

**Files to create/modify:**
- `kube/view/src/components/workspace/` — new workspace panel directory
  - `WorkspacePanel.vue` — main panel showing connected services
  - `WorkspaceConnect.vue` — connection wizard for each provider
  - `GoogleCalendarView.vue` — calendar list/CRUD
  - `ICloudView.vue` — notes + reminders + calendar views

**Existing files to extend:**
- `kube/view/src/components/auth/OAuthLoginButton.vue` — may need to add workspace connect buttons

**Steps:**
1. Build `WorkspacePanel.vue` — lists all connected services with status indicators
2. Per-service connection button that calls the Wails bindings (already wired)
3. Per-capability views using the existing Wails method patterns
4. Polling/message-passing for OAuth callback completion (reuses existing postMessage pattern from `workspace_handler.go`)

**Validation:** `npm run test` in `kube/view`

---

## Files Summary

### New files (10)
```
kube/backend/internal/workspace/gcal.go
kube/backend/internal/workspace/gcal_test.go
kube/backend/internal/workspace/icloud.go
kube/backend/internal/workspace/icloud_test.go
kube/backend/api/pkg/app_workspace_google_cal.go
kube/backend/api/pkg/app_workspace_google_cal_test.go
kube/backend/api/pkg/app_workspace_icloud.go
kube/view/src/components/workspace/WorkspacePanel.vue
kube/view/src/components/workspace/WorkspaceConnect.vue
kube/view/src/components/workspace/GoogleCalendarView.vue
```

### Modified files (3)
```
kube/backend/internal/workspace/google.go          — add calendar scope
kube/backend/internal/workspace/types.go           — add ServiceGCal, ServiceICloud, Calendarer interface
kube/backend/api/pkg/app_workspace_google.go       — wire gCal adapter singleton
```

### External credential files (user-managed)
```
~/.hermes/google_token.json                        — Hermes google-workspace skill
~/.hermes/google_client_secret.json                — Hermes google-workspace skill
.env / env vars                                    — ARGUS_GOOGLE_CLIENT_ID/SECRET (identity + workspace)
```

---

## Risks & Open Questions

1. **Three separate Google OAuth clients:** Identity sign-in, Workspace data access, and Hermes skill each need their own OAuth 2.0 Client ID in GCP. This is correct (separation of concerns) but means 3x setup overhead. Mitigation: all can live under the same GCP project.

2. **Google scope expansion UX:** Existing Workspace connections won't have Calendar after we add the scope. The disconnect/reconnect UX is acceptable per the existing code comment, but the UI should surface a clear "missing scopes" message.

3. **iCloud CalDAV complexity:** CalDAV is XML-based. Parsing iCal data manually is tedious. Mitigation: encode minimal XML payloads (query events, create events).

4. **iCloud app-specific password UX:** User must manually generate a password at appleid.apple.com. Less seamless than OAuth redirect but this is the same flow iCloud Mail users already accept.

5. **Cross-platform iCloud:** CalDAV works cross-platform. But `memo` and `remindctl` are macOS-only. On Linux/Windows, iCloud Notes and Reminders will be unavailable — only Calendar (via CalDAV).

---

## OAuth Client Summary

Three Google OAuth 2.0 Clients are needed (all can share one GCP project):

| Client | Type | Redirect URI | Scopes | Status |
|---|---|---|---|---|
| Argus Identity Sign-In | Desktop app | `http://127.0.0.1:8080/auth/google/callback` | `openid profile email` | Code DONE, needs GCP setup |
| Argus Workspace Data | Desktop app | `http://127.0.0.1:8080/workspace/oauth/callback` | `docs sheets tasks calendar userinfo` | Code DONE, needs GCP setup |
| Hermes google-workspace skill | Desktop app | `http://localhost:1/` (loopback) | `gmail calendar drive sheets docs` | Needs GCP setup + auth flow |

---

## Test Plan

| Phase | Test target | Command |
|---|---|---|
| 0 — Google Identity OAuth | End-to-end sign-in flow | Manual on macOS desktop build |
| 1 — GCal | Table-driven Go tests | `make test-go` in `kube/backend` |
| 2 — iCloud | Manual (requires real iCloud account) | Manual on macOS |
| 3 — Hermes skill setup | OAuth setup + credential check | `setup.py --check` |
| 4 — Frontend | Vitest component tests | `npm run test` in `kube/view` |

---

## Recommended Execution Order

1. **Phase 0 — Google Identity OAuth setup** (0.5 day). Lightest work — code exists, just needs GCP console config + env vars + end-to-end verification. Unblocks the "you can sign in" baseline.

2. **Phase 3 — Hermes google-workspace skill setup** (0.5 day). Run the setup script, obtain credentials, verify the agent can access Google services from CLI. Independent of app code.

3. **Phase 1 — Google Calendar adapter** (1-2 days). Lowest risk, follows established patterns exactly. Unblocks the "calendar" pillar immediately.

4. **Phase 2 — iCloud** (2-3 days). Most platform-specific. Calendar via CalDAV is the core value; Notes/Reminders piggyback on existing macOS CLI tools.

5. **Phase 4 — Frontend** (2-3 days). Wire everything together into a cohesive UI.

Total estimated effort: **6-9 days** for a single developer.
