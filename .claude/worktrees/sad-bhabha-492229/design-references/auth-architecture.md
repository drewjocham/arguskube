# KubeWatcher Auth Architecture

## 1. Method Audit — All 43 Methods Classified

### Read-Only (23)

| Method | Returns | Notes |
|--------|---------|-------|
| `GetAppMode` | `string` | |
| `GetClusterInfo` | `*k8s.ClusterInfo, error` | |
| `ListContexts` | `[]k8s.ContextInfo, error` | |
| `GetMetrics` | `*alerts.ClusterMetrics, error` | |
| `GetAlerts` | `[]alerts.Alert, error` | |
| `DiagnoseAlert` | `*ctxassembly.Bundle, error` | Gated by Pro |
| `GetPodLogs` | `[]alerts.LogLine, error` | |
| `GetDeploymentRevisions` | `[]k8s.DeploymentRevision, error` | |
| `StreamPodLogsFollow` | `[]string, error` | |
| `GetVPARecommendations` | `[]k8s.VPARecommendation, error` | |
| `GetFeatures` | `map[features.Feature]bool` | |
| `GetTier` | `config.Tier` | |
| `GetAnomalyJobs` | `[]anomaly.Job, error` | Gated by Pro |
| `GetChatHistory` | `[]ai.ChatEntry` | |
| `GetAutoSummary` | `*ai.AutoSummary` | |
| `GetAgentEventLog` | `[]ai.AgentEvent` | |
| `RunArgusScan` | `*popeye.Report, error` | Read-only computation |
| `QueryTimeSeriesMetrics` | `[]float64, error` | |
| `ListResources` | `*k8s.ResourceListResult, error` | |
| `GetResourceDetail` | `*k8s.ResourceDetailResult, error` | |
| `ListAllNamespaces` | `[]string, error` | |
| `ListNotebooks` | `[]notebooks.FileEntry, error` | |
| `ListIncidents` | `[]incidents.Incident` | |
| `GetTopology` | `*k8s.TopologyResult, error` | |
| `ListApplications` | `[]k8s.Application, error` | |
| `ListVulnerabilities` | `[]vulnscan.ScannedImage, error` | |
| `ListWorkflows` | `[]workflows.WorkflowSummary, error` | |
| `GetWorkflow` | `*workflows.Workflow, error` | |
| `GetNotebook` | `string, error` | |
| `ListRunbooks` | `[]runbooks.Runbook, error` | |
| `GetRunbook` | `string, error` | |
| `ConnectToAgent` | `[]agentconn.Anomaly, error` | |
| `GetAgentTopology` | `*agentconn.TopologyGraph, error` | |
| `QueryLogs` | `*k8s.LogQueryResult, error` | |
| `HandleURL` | void | Event emission only |
| `RunCodeSandbox` | `string, error` | Mock, no side-effects |
| `GetCodeSuggestion` | `string, error` | Mock, no side-effects |
| `CheckToolStatus` | `[]setup.ToolStatus` | |
| `LoginSaaS` | `string, error` | Returns mock token |
| `SendTerminalInput` | `error` | Terminal IO |
| `ResizeTerminal` | `error` | Terminal IO |
| `StartTerminal` | `error` | Terminal session |

### Mutating (10)

| Method | Effect | Permission |
|--------|--------|------------|
| `SwitchContext` | Changes active k8s context | `cluster:switch` |
| `DeletePod` | Deletes a pod | `workloads:delete` |
| `SaveNotebook` | Writes notebook file | `notebooks:write` |
| `DeleteNotebook` | Removes notebook | `notebooks:delete` |
| `CreateNotebookFolder` | Creates folder | `notebooks:write` |
| `MoveNotebook` | Moves/renames notebook | `notebooks:write` |
| `SaveRunbook` | Writes runbook | `runbooks:write` |
| `DeleteRunbook` | Removes runbook | `runbooks:delete` |
| `CreateRunbook` | Creates runbook | `runbooks:write` |
| `CreateIncident` | Creates incident | `incidents:write` |
| `UpdateIncident` | Edits incident | `incidents:write` |
| `DeleteIncident` | Removes incident | `incidents:delete` |
| `SyncApplication` | Restarts deployment | `workloads:sync` |
| `ScanImage` | Triggers vulnerability scan | `vulnerability:scan` |
| `ScanAllImages` | Triggers full scan | `vulnerability:scan` |
| `SaveWorkflow` | Creates/updates workflow | `workflows:write` |
| `DeleteWorkflow` | Removes workflow | `workflows:delete` |
| `SendChatMessage` | Modifies AI chat history | `ai:chat` |

### Admin (4)

| Method | Effect | Permission |
|--------|--------|------------|
| `InstallArgusScan` | Installs system binary | `admin:tools:install` |
| `DeployAgent` | Deploys cluster DaemonSet | `admin:agent:deploy` |
| `UndeployAgent` | Removes cluster DaemonSet | `admin:agent:undeploy` |
| `TestS3Connection` | Tests S3 credentials | `admin:config:test` |

---

## 2. Role–Permission Matrix

```
┌──────────────┬─────────────────────────────────────────┐
│    Role      │              Permissions                │
├──────────────┼─────────────────────────────────────────┤
│              │  All read-only methods                  │
│   viewer     │  LoginSaaS, GetFeatures, GetTier        │
│              │  StartTerminal (read-only)              │
├──────────────┼─────────────────────────────────────────┤
│              │  Everything viewer has                  │
│              │  +                                      │
│   operator   │  SwitchContext, DeletePod               │
│              │  Notebooks CRUD                         │
│              │  Runbooks CRUD                          │
│              │  Incidents CRUD                         │
│              │  SyncApplication                        │
│              │  ScanImage, ScanAllImages               │
│              │  Workflows CRUD                         │
│              │  SendChatMessage                        │
│              │  RunCodeSandbox, GetCodeSuggestion      │
│              │  SendTerminalInput                      │
├──────────────┼─────────────────────────────────────────┤
│              │  Everything operator has                │
│   admin      │  +                                      │
│              │  InstallArgusScan                       │
│              │  DeployAgent, UndeployAgent             │
│              │  TestS3Connection                       │
│              │  SwitchContext (any context)             │
└──────────────┴─────────────────────────────────────────┘
```

---

## 3. Package Skeleton — `backend/internal/auth/`

### `backend/internal/auth/models.go`

```go
package auth

import "time"

type Provider string

const (
	ProviderNone    Provider = "none"
	ProviderBuiltin Provider = "builtin"
	ProviderGoogle  Provider = "google"
)

type Identity struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Provider   Provider  `json:"provider"`
	ProviderID string    `json:"providerId"`
	Roles      []Role    `json:"roles"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Credentials struct {
	Email    string   `json:"email"`
	Password string   `json:"password,omitempty"`
	Provider Provider `json:"provider,omitempty"`
	Code     string   `json:"code,omitempty"`
}

type Session struct {
	ID         string    `json:"id"`
	IdentityID string    `json:"identityId"`
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

type RegisterRequest struct {
	Email    string   `json:"email"`
	Password string   `json:"password,omitempty"`
	Provider Provider `json:"provider,omitempty"`
	Name     string   `json:"name,omitempty"`
}

type AuthResult struct {
	Identity *Identity `json:"identity"`
	Session  *Session  `json:"session"`
}
```

### `backend/internal/auth/idp.go`

```go
package auth

import "context"

type IdentityProvider interface {
	ValidateToken(ctx context.Context, rawToken string) (*Identity, error)
	Authenticate(ctx context.Context, provider Provider, code string) (*Identity, error)
	GetOAuthURL(ctx context.Context, provider Provider) (string, error)
}

type UserStore interface {
	Create(ctx context.Context, identity *Identity, password string) error
	GetByEmail(ctx context.Context, email string) (*Identity, error)
	GetByProviderID(ctx context.Context, provider Provider, providerID string) (*Identity, error)
	GetByID(ctx context.Context, id string) (*Identity, error)
	Update(ctx context.Context, identity *Identity) error
	Delete(ctx context.Context, id string) error
	VerifyPassword(ctx context.Context, email, password string) bool
	UpdatePassword(ctx context.Context, id, newPassword string) error
}

type SessionStore interface {
	Create(ctx context.Context, session *Session) error
	GetByToken(ctx context.Context, token string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteAllForUser(ctx context.Context, identityID string) error
}
```

### `backend/internal/auth/rbac.go`

```go
package auth

import "context"

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

type Resource string

const (
	ResourceCluster       Resource = "cluster"
	ResourceWorkloads     Resource = "workloads"
	ResourceIncidents     Resource = "incidents"
	ResourceNotebooks     Resource = "notebooks"
	ResourceRunbooks      Resource = "runbooks"
	ResourceWorkflows     Resource = "workflows"
	ResourceVulnerability Resource = "vulnerability"
	ResourceTopology      Resource = "topology"
	ResourceMetrics       Resource = "metrics"
	ResourceAI            Resource = "ai"
	ResourceTerminal      Resource = "terminal"
	ResourceCode          Resource = "code"
	ResourceAdmin         Resource = "admin"
)

type Action string

const (
	ActionView   Action = "view"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionScan   Action = "scan"
	ActionSync   Action = "sync"
	ActionSwitch Action = "switch"
	ActionChat   Action = "chat"
	ActionStart  Action = "start"
	ActionInput  Action = "input"
	ActionInstall Action = "install"
	ActionDeploy Action = "deploy"
	ActionTest   Action = "test"
)

var defaultPermissions = map[Role]map[string]bool{
	RoleViewer: {
		"cluster:view":        true,
		"workloads:view":      true,
		"incidents:view":      true,
		"notebooks:view":      true,
		"runbooks:view":       true,
		"workflows:view":      true,
		"vulnerability:view":  true,
		"topology:view":       true,
		"metrics:view":        true,
		"ai:view":             true,
		"terminal:start":      true,
	},
	RoleOperator: {
		"cluster:view":        true,
		"cluster:switch":      true,
		"workloads:view":      true,
		"workloads:delete":    true,
		"workloads:sync":      true,
		"incidents:view":      true,
		"incidents:write":     true,
		"incidents:delete":    true,
		"notebooks:view":      true,
		"notebooks:write":     true,
		"notebooks:delete":    true,
		"runbooks:view":       true,
		"runbooks:write":      true,
		"runbooks:delete":     true,
		"workflows:view":      true,
		"workflows:write":     true,
		"workflows:delete":    true,
		"vulnerability:view":  true,
		"vulnerability:scan":  true,
		"topology:view":       true,
		"metrics:view":        true,
		"ai:view":             true,
		"ai:chat":             true,
		"terminal:start":      true,
		"terminal:input":      true,
		"code:sandbox":        true,
	},
	RoleAdmin: {
		"*": true,
	},
}

type RoleManager interface {
	HasPermission(identity *Identity, resource Resource, action Action) bool
	GetRoles(identity *Identity) []Role
	AssignRole(identity *Identity, role Role) error
}
```

### `backend/internal/auth/errors.go`

```go
package auth

import "errors"

var (
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrInvalidEmail    = errors.New("invalid email format")
	ErrWeakPassword    = errors.New("password does not meet requirements")
	ErrEmailTaken      = errors.New("email already registered")
	ErrInvalidProvider = errors.New("unsupported identity provider")
	ErrOAuthState      = errors.New("oauth state mismatch")
	ErrTokenExchange   = errors.New("token exchange failed")
	ErrUserNotFound    = errors.New("user not found")
	ErrWrongPassword   = errors.New("incorrect password")
	ErrSessionExpired  = errors.New("session expired")
	ErrForbidden       = errors.New("insufficient permissions")
	ErrUserStore       = errors.New("user store operation failed")
)
```

### `backend/internal/auth/middleware.go`

```go
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const identityKey contextKey = "identity"

type Middleware struct {
	logger        *slog.Logger
	idp           IdentityProvider
	roleManager   RoleManager
	ExcludedPaths []string
}

func NewMiddleware(logger *slog.Logger, idp IdentityProvider, rm RoleManager) *Middleware {
	return &Middleware{
		logger:        logger,
		idp:           idp,
		roleManager:   rm,
		ExcludedPaths: []string{},
	}
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		for _, excluded := range m.ExcludedPaths {
			if strings.HasPrefix(path, excluded) {
				next.ServeHTTP(w, r)
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		identity, err := m.idp.ValidateToken(r.Context(), token)
		if err != nil {
			m.logger.WarnContext(r.Context(), "auth failed",
				slog.String("error", err.Error()),
			)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), identityKey, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	identity, ok := ctx.Value(identityKey).(*Identity)
	return identity, ok
}

func RequirePermission(rm RoleManager, resource Resource, action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !rm.HasPermission(identity, resource, action) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

---

## 4. Config Changes — `backend/internal/config/config.go`

Add to `Config`:

```go
type Config struct {
	// ... existing fields ...
	Auth AuthConfig
}
```

New struct:

```go
type AuthConfig struct {
	Provider     string        `env:"KW_AUTH_PROVIDER"`
	ClientID     string        `env:"KW_AUTH_CLIENT_ID"`
	ClientSecret string        `env:"KW_AUTH_CLIENT_SECRET"`
	CallbackURL  string        `env:"KW_AUTH_CALLBACK_URL"`
	JWKSEndpoint string        `env:"KW_AUTH_JWKS_ENDPOINT"`
	SessionTTL   time.Duration
}
```

Initialize in `New()`:

```go
Auth: AuthConfig{
	Provider:     env("KW_AUTH_PROVIDER", "none"),
	ClientID:     env("KW_AUTH_CLIENT_ID", ""),
	ClientSecret: env("KW_AUTH_CLIENT_SECRET", ""),
	CallbackURL:  env("KW_AUTH_CALLBACK_URL", "http://localhost:8080/auth/callback"),
	JWKSEndpoint: env("KW_AUTH_JWKS_ENDPOINT", ""),
	SessionTTL:   24 * time.Hour,
},
```

---

## 5. File Creation Order

| Step | File | Action |
|------|------|--------|
| 1 | `backend/internal/auth/models.go` | Create |
| 2 | `backend/internal/auth/errors.go` | Create |
| 3 | `backend/internal/auth/idp.go` | Create |
| 4 | `backend/internal/auth/rbac.go` | Create |
| 5 | `backend/internal/config/config.go` | Modify (add `AuthConfig`) |
| 6 | `backend/internal/auth/middleware.go` | Create |

Each file is **interface-only / type-only** — zero runtime cost, fully compilable. The pluggability contract is now set; any IdP (Google, Okta, builtin) just needs to satisfy `IdentityProvider` and `UserStore`.

---

## 6. Registration Flow — With Google / With Email

### Registration with Google (OAuth)

```
User clicks "Sign in with Google"
  → App calls GetOAuthURL("google")
  → Redirect user to Google consent screen
  → Google redirects to CallbackURL with ?code=...
  → App calls Authenticate(ctx, "google", code)
    → Exchange code for token (POST /token)
    → Fetch userinfo from Google (GET /userinfo)
    → Look up existing user by providerID
    → If new: create Identity via UserStore.Create
    → If existing: update Identity
    → Create Session via SessionStore.Create
    → Return AuthResult{Identity, Session}
```

### Registration with Email/Password

```
User fills email + password form
  → App calls RegisterWithEmail(email, password)
    → Validate email format
    → Validate password strength (min 8 chars, etc.)
    → Check UserStore.GetByEmail — if exists, return ErrEmailTaken
    → Hash password (bcrypt)
    → Create Identity via UserStore.Create
    → Create Session via SessionStore.Create
    → Return AuthResult{Identity, Session}
```

---

## 7. Logging — Event Map

### Email Registration

| Step | Level | Event | Fields |
|------|-------|-------|--------|
| Request received | `Debug` | `"register request received"` | email |
| Email validated | `Warn` if fail | `"email validation failed"` | email, reason |
| Password validated | `Warn` if fail | `"password validation failed"` | email, reason |
| Duplicate check | `Info` | `"checking existing user"` | email |
| Duplicate found | `Warn` | `"registration failed — email already registered"` | email |
| User creation | `Info` | `"creating user"` | email |
| Password hashed | `Debug` | `"password hashed"` | (no sensitive data) |
| Persisted | `Info` / `Error` | `"user persisted"` / `"user persistence failed"` | email, error |
| Session created | `Info` | `"session created"` | userId, sessionId |
| Success | `Info` | `"registration successful"` | email, userId |

### Google Registration

| Step | Level | Event | Fields |
|------|-------|-------|--------|
| OAuth URL generated | `Debug` | `"generating oauth url"` | provider |
| Redirect initiated | `Info` | `"redirecting to google oauth"` | provider |
| Callback received | `Debug` | `"oauth callback received"` | state |
| State validated | `Warn` if fail | `"oauth state validated"` / `"oauth state mismatch"` | expected, received |
| Token exchange | `Info` | `"exchanging auth code for token"` | |
| Exchange failed | `Error` | `"token exchange failed"` | error |
| Userinfo fetched | `Info` | `"fetching user info from google"` | |
| Userinfo parsed | `Info` | `"google user info received"` | email, googleUserId |
| Account linked | `Info` | `"linking google account to existing user"` | email |
| Account created | `Info` | `"creating user from google account"` | email |
| Success | `Info` | `"google registration successful"` | email, userId, provider |

---

## 8. Account Management Endpoints

| Action | Method | Permission | Description |
|--------|--------|------------|-------------|
| `GetProfile` | Read | `auth:profile:view` | Returns current user's identity |
| `UpdateProfile` | Mutate | `auth:profile:write` | Update name, email |
| `ChangePassword` | Mutate | `auth:profile:write` | Change password (email users only) |
| `DeleteAccount` | Mutate | `auth:profile:delete` | Delete account and all sessions |
| `ListSessions` | Read | `auth:sessions:view` | List active sessions |
| `RevokeSession` | Mutate | `auth:sessions:revoke` | Revoke a specific session |
| `GetAuditLog` | Read | `auth:audit:view` | View account activity log |
