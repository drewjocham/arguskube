package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// app_vault — Settings → Vault.
//
// The Vault gives the user one place to see the auth/credential state for
// every external system the app talks to (GitHub, Google OAuth, OIDC,
// ArgoCD, DeepSeek, Snyk, Slack, Notion, Confluence, Azure, GitLab, …)
// plus a small key/value store for application-managed custom secrets.
//
// Status semantics returned by GetVaultStatus:
//
//	"missing"     — no value configured
//	"present"     — value is set; we haven't probed it (cheap path)
//	"valid"       — last live probe succeeded (currently only GitHub)
//	"expired"     — server told us the token is no longer authoritative
//	"invalid"     — server rejected the credential outright
//	"error"       — probe failed for an unrelated reason (network, etc.)
//
// Live-probe is opt-in via TestVaultProvider so a Settings panel render
// doesn't fan out N HTTP calls every time it mounts. GetVaultStatus
// returns presence-only by default and reads the cached probe result
// for any provider where TestVaultProvider has been called recently.

// --- Types exposed to the frontend -----------------------------------------

type VaultEntry struct {
	ID                 string `json:"id"`
	Label              string `json:"label"`
	Kind               string `json:"kind"`               // "oauth" | "token" | "credential"
	Status             string `json:"status"`             // see semantics above
	Message            string `json:"message,omitempty"`  // human-friendly detail
	Configured         bool   `json:"configured"`         // value is set on disk
	Probable           bool   `json:"probable"`           // we know how to live-probe it
	ConfigureAnchor    string `json:"configureAnchor"`    // deep-link target inside Settings
	LastCheckedAt      string `json:"lastCheckedAt,omitempty"`
}

type VaultSecret struct {
	Key       string    `json:"key"`
	ValueMask string    `json:"valueMask"` // masked for display; never leaks the raw secret
	Notes     string    `json:"notes,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// --- Status registry -------------------------------------------------------

// vaultProbeCache holds the most recent live-probe result for a provider so
// repeated GetVaultStatus calls don't re-issue HTTP. Caller refreshes via
// TestVaultProvider(id).
type vaultProbeResult struct {
	Status      string
	Message     string
	CheckedAt   time.Time
}

var (
	vaultProbeMu    sync.RWMutex
	vaultProbeCache = map[string]vaultProbeResult{}
)

func vaultGetProbe(id string) (vaultProbeResult, bool) {
	vaultProbeMu.RLock()
	defer vaultProbeMu.RUnlock()
	r, ok := vaultProbeCache[id]
	return r, ok
}

func vaultSetProbe(id string, r vaultProbeResult) {
	vaultProbeMu.Lock()
	vaultProbeCache[id] = r
	vaultProbeMu.Unlock()
}

// providerSpec describes one credential row. Built once per call so the
// frontend gets a stable, sorted list.
type providerSpec struct {
	id              string
	label           string
	kind            string // oauth | token | credential
	hasValue        func(c *configMirror) bool
	probable        bool
	configureAnchor string
}

// configMirror is a thin pull-through to a.cfg used by the spec table — keeps
// the spec list declarative without dragging the config import into every line.
type configMirror struct {
	app *App
}

// GetVaultStatus enumerates every credential the app cares about and reports
// presence + (when available) the cached live-probe state.
func (a *App) GetVaultStatus() ([]VaultEntry, error) {
	if a.cfg == nil {
		return []VaultEntry{}, nil
	}
	cm := &configMirror{app: a}
	specs := vaultProviderSpecs()

	out := make([]VaultEntry, 0, len(specs))
	for _, s := range specs {
		entry := VaultEntry{
			ID:              s.id,
			Label:           s.label,
			Kind:            s.kind,
			Configured:      s.hasValue(cm),
			Probable:        s.probable,
			ConfigureAnchor: s.configureAnchor,
		}
		if !entry.Configured {
			entry.Status = "missing"
			entry.Message = "Not configured."
		} else {
			entry.Status = "present"
			entry.Message = "Configured. Click Test to verify."
		}
		// Overlay any cached probe result.
		if cached, ok := vaultGetProbe(s.id); ok {
			entry.Status = cached.Status
			entry.Message = cached.Message
			entry.LastCheckedAt = cached.CheckedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// TestVaultProvider runs a live probe for the given provider id (where
// supported) and updates the in-memory cache. Returns the resulting entry so
// the UI can re-render that single row without round-tripping the whole list.
func (a *App) TestVaultProvider(id string) (VaultEntry, error) {
	if a.cfg == nil {
		return VaultEntry{}, fmt.Errorf("config not loaded")
	}
	cm := &configMirror{app: a}
	var spec *providerSpec
	for _, s := range vaultProviderSpecs() {
		if s.id == id {
			ss := s
			spec = &ss
			break
		}
	}
	if spec == nil {
		return VaultEntry{}, fmt.Errorf("unknown provider: %q", id)
	}

	if !spec.hasValue(cm) {
		res := vaultProbeResult{Status: "missing", Message: "Not configured.", CheckedAt: time.Now()}
		vaultSetProbe(id, res)
		return entryFromProbe(spec, cm, res), nil
	}
	if !spec.probable {
		res := vaultProbeResult{
			Status:    "present",
			Message:   "No live test available for this provider yet.",
			CheckedAt: time.Now(),
		}
		vaultSetProbe(id, res)
		return entryFromProbe(spec, cm, res), nil
	}

	res := a.runVaultProbe(a.ctx, id)
	vaultSetProbe(id, res)
	return entryFromProbe(spec, cm, res), nil
}

func entryFromProbe(s *providerSpec, cm *configMirror, r vaultProbeResult) VaultEntry {
	return VaultEntry{
		ID:              s.id,
		Label:           s.label,
		Kind:            s.kind,
		Status:          r.Status,
		Message:         r.Message,
		Configured:      s.hasValue(cm),
		Probable:        s.probable,
		ConfigureAnchor: s.configureAnchor,
		LastCheckedAt:   r.CheckedAt.UTC().Format(time.RFC3339),
	}
}

// --- Provider spec table ---------------------------------------------------

func vaultProviderSpecs() []providerSpec {
	return []providerSpec{
		// AI / LLM
		{
			id: "deepseek", label: "DeepSeek API", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.AI.DeepSeekAPIKey) != "" },
			configureAnchor: "ai-integrations",
		},
		// Source / CI providers
		{
			id: "github", label: "GitHub", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.GitHubToken) != "" },
			probable:        true,
			configureAnchor: "pipelines-github",
		},
		{
			id: "gitlab", label: "GitLab", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.GitLabToken) != "" },
			configureAnchor: "pipelines-gitlab",
		},
		{
			id: "azure-pipelines", label: "Azure Pipelines", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.AzureToken) != "" },
			configureAnchor: "pipelines-azure",
		},
		{
			id: "circleci", label: "CircleCI", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.CircleCIToken) != "" },
			configureAnchor: "pipelines-circleci",
		},
		{
			id: "aws-codebuild", label: "AWS CodeBuild", kind: "credential",
			hasValue: func(c *configMirror) bool {
				return strings.TrimSpace(c.app.cfg.Pipelines.AWSAccessKey) != "" &&
					strings.TrimSpace(c.app.cfg.Pipelines.AWSSecretKey) != ""
			},
			configureAnchor: "pipelines-aws",
		},
		{
			id: "gcp-cloudbuild", label: "Google Cloud Build", kind: "credential",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.GCPProject) != "" },
			configureAnchor: "pipelines-gcp",
		},
		// Knowledge / docs destinations
		{
			id: "notion", label: "Notion", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.NotionToken) != "" },
			configureAnchor: "auto-code-review",
		},
		{
			id: "confluence", label: "Confluence", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Pipelines.ConfluenceToken) != "" },
			configureAnchor: "auto-code-review",
		},
		// Cluster integrations
		{
			id: "argocd", label: "ArgoCD", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.ArgoCD.Token) != "" },
			configureAnchor: "arguscd-section",
		},
		{
			id: "snyk", label: "Snyk", kind: "token",
			hasValue:        func(c *configMirror) bool { return strings.TrimSpace(c.app.cfg.Security.SnykToken) != "" },
			configureAnchor: "security-tools",
		},
		// Sign-in / OAuth providers — surfaced even when unconfigured so the
		// user knows what's available.
		{
			id: "google-oauth", label: "Google (sign-in)", kind: "oauth",
			hasValue: func(c *configMirror) bool {
				return strings.TrimSpace(c.app.cfg.Auth.GoogleClientID) != "" &&
					strings.TrimSpace(c.app.cfg.Auth.GoogleClientSecret) != ""
			},
			configureAnchor: "auth-providers",
		},
		{
			id: "oidc", label: "OIDC SSO", kind: "oauth",
			hasValue: func(c *configMirror) bool {
				return strings.TrimSpace(c.app.cfg.Auth.OIDCIssuer) != "" &&
					strings.TrimSpace(c.app.cfg.Auth.OIDCClientID) != ""
			},
			configureAnchor: "auth-providers",
		},
	}
}

// --- Live probes -----------------------------------------------------------

func (a *App) runVaultProbe(ctx context.Context, id string) vaultProbeResult {
	switch id {
	case "github":
		return a.probeGitHub(ctx)
	default:
		return vaultProbeResult{
			Status:    "present",
			Message:   "No live test available for this provider yet.",
			CheckedAt: time.Now(),
		}
	}
}

func (a *App) probeGitHub(ctx context.Context) vaultProbeResult {
	now := time.Now()
	token := strings.TrimSpace(a.cfg.Pipelines.GitHubToken)
	if token == "" {
		return vaultProbeResult{Status: "missing", Message: "Token is empty.", CheckedAt: now}
	}
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return vaultProbeResult{Status: "error", Message: err.Error(), CheckedAt: now}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "argus")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return vaultProbeResult{Status: "error", Message: "network error: " + err.Error(), CheckedAt: now}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body struct {
			Login string `json:"login"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		msg := "Token is valid"
		if body.Login != "" {
			msg = "Authenticated as " + body.Login
		}
		// GitHub fine-grained PATs include their expiry in this header.
		if exp := resp.Header.Get("github-authentication-token-expiration"); exp != "" {
			msg += " · expires " + exp
		}
		a.logger.InfoContext(ctx, "vault: github probe ok", slog.String("login", body.Login))
		return vaultProbeResult{Status: "valid", Message: msg, CheckedAt: now}
	case http.StatusUnauthorized, http.StatusForbidden:
		// Differentiate "rejected" from "expired": GitHub uses 401 for both,
		// but `X-GitHub-SSO` and the body usually identify expired tokens.
		// We classify as "expired" only when the message strongly suggests it.
		body := readBodyMax(resp.Body, 240)
		if strings.Contains(strings.ToLower(body), "expired") {
			return vaultProbeResult{Status: "expired", Message: "GitHub says the token is expired.", CheckedAt: now}
		}
		return vaultProbeResult{Status: "invalid", Message: "GitHub rejected the token (HTTP " + resp.Status + ").", CheckedAt: now}
	case http.StatusNotFound:
		return vaultProbeResult{Status: "invalid", Message: "GitHub returned 404 — token may be scoped to a different account.", CheckedAt: now}
	default:
		return vaultProbeResult{
			Status:    "error",
			Message:   fmt.Sprintf("Unexpected response: HTTP %d", resp.StatusCode),
			CheckedAt: now,
		}
	}
}

func readBodyMax(r interface{ Read([]byte) (int, error) }, max int) string {
	buf := make([]byte, max)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

// --- Custom-secrets store --------------------------------------------------
//
// User-managed key/value entries. Stored unencrypted on disk under the
// user's $HOME — same trust model the rest of the app uses for tokens.
// Mode 0600 so only the user can read it. The raw value never leaves the
// backend except through TestVaultProvider; ListVaultSecrets returns a
// masked variant so the UI can show "is something there?" without
// echoing the secret in screenshots / logs.

const vaultSecretsMaxKey = 96
const vaultSecretsMaxValue = 16 * 1024

type storedSecret struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Notes     string    `json:"notes,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func vaultSecretsPath() (string, error) {
	dir := filepath.Join(os.ExpandEnv("$HOME"), ".argus", "vault")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create vault dir: %w", err)
	}
	return filepath.Join(dir, "secrets.json"), nil
}

func loadVaultSecrets() ([]storedSecret, error) {
	p, err := vaultSecretsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return []storedSecret{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []storedSecret
	if len(b) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse vault secrets: %w", err)
	}
	return out, nil
}

func saveVaultSecrets(items []storedSecret) error {
	p, err := vaultSecretsPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// ListVaultSecrets returns every custom secret with the value masked. Sort
// by updatedAt desc so the most-recently-touched entries float to the top.
func (a *App) ListVaultSecrets() ([]VaultSecret, error) {
	items, err := loadVaultSecrets()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	out := make([]VaultSecret, 0, len(items))
	for _, it := range items {
		out = append(out, VaultSecret{
			Key:       it.Key,
			ValueMask: maskSecret(it.Value),
			Notes:     it.Notes,
			UpdatedAt: it.UpdatedAt,
		})
	}
	return out, nil
}

// SetVaultSecret inserts or updates a custom secret. Empty value is a no-op
// guarded by the frontend; we re-validate here so a stray empty payload
// doesn't silently overwrite the entry.
func (a *App) SetVaultSecret(key, value, notes string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if len(key) > vaultSecretsMaxKey {
		return fmt.Errorf("key too long (max %d)", vaultSecretsMaxKey)
	}
	if value == "" {
		return fmt.Errorf("value is required (use DeleteVaultSecret to remove)")
	}
	if len(value) > vaultSecretsMaxValue {
		return fmt.Errorf("value too long (max %d bytes)", vaultSecretsMaxValue)
	}

	items, err := loadVaultSecrets()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updated := false
	for i := range items {
		if items[i].Key == key {
			items[i].Value = value
			items[i].Notes = notes
			items[i].UpdatedAt = now
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, storedSecret{
			Key:       key,
			Value:     value,
			Notes:     notes,
			UpdatedAt: now,
		})
	}
	return saveVaultSecrets(items)
}

// DeleteVaultSecret removes a custom secret by key. Missing keys are not
// an error — the desired post-state is the same.
func (a *App) DeleteVaultSecret(key string) error {
	items, err := loadVaultSecrets()
	if err != nil {
		return err
	}
	out := make([]storedSecret, 0, len(items))
	for _, it := range items {
		if it.Key != key {
			out = append(out, it)
		}
	}
	return saveVaultSecrets(out)
}
