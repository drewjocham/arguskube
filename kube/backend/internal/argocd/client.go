package argocd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Config holds connection details for an Argo CD server.
type Config struct {
	URL      string // e.g. https://argocd.example.com
	Token    string // API bearer token
	Insecure bool   // skip TLS verification (common for in-cluster)
}

// Client talks to the Argo CD REST API v1.
type Client struct {
	cfg    Config
	http   *http.Client
	logger *slog.Logger
}

// New creates an Argo CD API client. Returns nil if URL is empty.
func New(cfg Config, logger *slog.Logger) *Client {
	if cfg.URL == "" {
		return nil
	}

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-configured
	}

	return &Client{
		cfg: cfg,
		http: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
		logger: logger.With("component", "argocd"),
	}
}

// Connected returns true if the client has a non-empty URL configured.
func (c *Client) Connected() bool {
	return c != nil && c.cfg.URL != ""
}

// ── Argo CD API types ──────────────────────────────────────────────

// App is the Argus representation of an Argo CD Application.
type App struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Project       string            `json:"project"`
	SyncStatus    string            `json:"syncStatus"`   // Synced, OutOfSync, Unknown
	HealthStatus  string            `json:"healthStatus"` // Healthy, Degraded, Progressing, Missing, Unknown
	Replicas      int32             `json:"replicas"`
	ReadyReplicas int32             `json:"readyReplicas"`
	Image         string            `json:"image"`
	LastSync      string            `json:"lastSync"`
	RepoURL       string            `json:"repoUrl"`
	Path          string            `json:"path"`
	TargetRev     string            `json:"targetRevision"`
	DestServer    string            `json:"destServer"`
	DestNamespace string            `json:"destNamespace"`
	CreatedAt     string            `json:"createdAt"`
	Resources     []AppResource     `json:"resources,omitempty"`
	History       []AppHistoryEntry `json:"history,omitempty"`
}

// AppHistoryEntry is one deploy in an Argo CD Application's revision history.
type AppHistoryEntry struct {
	ID         int64  `json:"id"`
	Revision   string `json:"revision"`
	DeployedAt string `json:"deployedAt"`
	Source     string `json:"source,omitempty"`
}

// AppResource is a single managed resource inside an Argo CD Application.
type AppResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"` // Synced, OutOfSync
	Health    string `json:"health"` // Healthy, Degraded, Progressing, Missing
	Group     string `json:"group"`
	Version   string `json:"version"`
}

// AppDiff represents a diff between live and desired state.
type AppDiff struct {
	Resource string `json:"resource"` // e.g. "apps/Deployment/nginx"
	Live     string `json:"live"`     // live manifest YAML
	Target   string `json:"target"`   // desired manifest YAML
	Diff     string `json:"diff"`     // unified diff text
}

// SyncResult holds the outcome of a sync operation.
type SyncResult struct {
	Phase   string `json:"phase"` // Succeeded, Failed, Running
	Message string `json:"message"`
}

// ── List Applications ───────���──────────────────────────────────────

func (c *Client) ListApps(ctx context.Context, project string) ([]App, error) {
	path := "/api/v1/applications"
	if project != "" {
		path += "?project=" + project
	}

	var raw argoAppList
	if err := c.get(ctx, path, &raw); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}

	apps := make([]App, 0, len(raw.Items))
	for _, item := range raw.Items {
		apps = append(apps, mapApp(item))
	}
	return apps, nil
}

// ── Get Application Detail ─────────────────────────────────────────

func (c *Client) GetApp(ctx context.Context, name string) (*App, error) {
	var raw argoApp
	if err := c.get(ctx, "/api/v1/applications/"+name, &raw); err != nil {
		return nil, fmt.Errorf("get application %q: %w", name, err)
	}
	app := mapApp(raw)
	return &app, nil
}

// ── Get Application Resources (resource tree) ──────────────────────

func (c *Client) GetAppResources(ctx context.Context, name string) ([]AppResource, error) {
	var raw struct {
		Nodes []struct {
			Group           string                   `json:"group"`
			Version         string                   `json:"version"`
			Kind            string                   `json:"kind"`
			Namespace       string                   `json:"namespace"`
			Name            string                   `json:"name"`
			Health          *struct{ Status string } `json:"health"`
			ResourceVersion string                   `json:"resourceVersion"`
		} `json:"nodes"`
	}
	if err := c.get(ctx, "/api/v1/applications/"+name+"/resource-tree", &raw); err != nil {
		return nil, fmt.Errorf("get resource tree: %w", err)
	}

	resources := make([]AppResource, 0, len(raw.Nodes))
	for _, n := range raw.Nodes {
		health := ""
		if n.Health != nil {
			health = n.Health.Status
		}
		resources = append(resources, AppResource{
			Kind:      n.Kind,
			Name:      n.Name,
			Namespace: n.Namespace,
			Health:    health,
			Group:     n.Group,
			Version:   n.Version,
		})
	}
	return resources, nil
}

// ── Get Application Diffs (managed-resources with diff) ────────────

func (c *Client) GetAppDiffs(ctx context.Context, name string) ([]AppDiff, error) {
	var raw struct {
		Items []struct {
			Group       string `json:"group"`
			Kind        string `json:"kind"`
			Name        string `json:"name"`
			Namespace   string `json:"namespace"`
			LiveState   string `json:"liveState"`
			TargetState string `json:"targetState"`
			Diff        struct {
				NormalizedLiveState string `json:"normalizedLiveState"`
				PredictedLiveState  string `json:"predictedLiveState"`
			} `json:"diff"`
		} `json:"items"`
	}
	if err := c.get(ctx, "/api/v1/applications/"+name+"/managed-resources", &raw); err != nil {
		return nil, fmt.Errorf("get managed resources: %w", err)
	}

	var diffs []AppDiff
	for _, item := range raw.Items {
		if item.LiveState == item.TargetState {
			continue // no drift
		}
		if item.LiveState == "" || item.TargetState == "" || item.LiveState == "null" || item.TargetState == "null" {
			continue
		}
		diffs = append(diffs, AppDiff{
			Resource: fmt.Sprintf("%s/%s/%s", item.Group, item.Kind, item.Name),
			Live:     item.LiveState,
			Target:   item.TargetState,
		})
	}
	return diffs, nil
}

// ── Sync Application ───────────────────────────────────────────────

func (c *Client) SyncApp(ctx context.Context, name string) (*SyncResult, error) {
	body := `{"prune":false,"dryRun":false,"strategy":{"hook":{}}}`
	var raw struct {
		Phase   string `json:"phase"`
		Message string `json:"message"`
	}
	if err := c.post(ctx, "/api/v1/applications/"+name+"/sync", body, &raw); err != nil {
		return nil, fmt.Errorf("sync application %q: %w", name, err)
	}
	return &SyncResult{Phase: raw.Phase, Message: raw.Message}, nil
}

// ── Rollback Application ────────────��──────────────────────────────

func (c *Client) RollbackApp(ctx context.Context, name string, revisionID int64) error {
	body := fmt.Sprintf(`{"id":%d}`, revisionID)
	if err := c.post(ctx, "/api/v1/applications/"+name+"/rollback", body, nil); err != nil {
		return fmt.Errorf("rollback application %q: %w", name, err)
	}
	return nil
}

// ── Refresh Application ──────��─────────────────────────────────────

func (c *Client) RefreshApp(ctx context.Context, name string, hard bool) error {
	refreshType := "normal"
	if hard {
		refreshType = "hard"
	}
	return c.get(ctx, "/api/v1/applications/"+name+"?refresh="+refreshType, nil)
}

// ── List Projects ──────────────────────────────────────────────────

// ListProjects returns the names of Argo CD AppProjects available on the
// server. Used to populate a project filter dropdown in the UI.
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	var raw struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := c.get(ctx, "/api/v1/projects", &raw); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	names := make([]string, 0, len(raw.Items))
	for _, p := range raw.Items {
		if p.Metadata.Name != "" {
			names = append(names, p.Metadata.Name)
		}
	}
	return names, nil
}

// ── Test Connection ──────────���───────────────────────────────���─────

func (c *Client) TestConnection(ctx context.Context) error {
	var raw struct {
		Items []interface{} `json:"items"`
	}
	return c.get(ctx, "/api/v1/applications?limit=1", &raw)
}

// ── Internal helpers ───────────────────────────────────────────────

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	url := strings.TrimRight(c.cfg.URL, "/") + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("build GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("argocd request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("argocd %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path, body string, out interface{}) error {
	url := strings.TrimRight(c.cfg.URL, "/") + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("build POST request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("argocd request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("argocd %s returned %d: %s", path, resp.StatusCode, string(respBody))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ── Argo CD raw API structures ───────���─────────────────────────────

type argoAppList struct {
	Items []argoApp `json:"items"`
}

type argoApp struct {
	Metadata struct {
		Name              string `json:"name"`
		Namespace         string `json:"namespace"`
		CreationTimestamp string `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Project string `json:"project"`
		Source  struct {
			RepoURL        string `json:"repoURL"`
			Path           string `json:"path"`
			TargetRevision string `json:"targetRevision"`
		} `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status string `json:"status"`
		} `json:"sync"`
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		Summary struct {
			Images []string `json:"images"`
		} `json:"summary"`
		OperationState *struct {
			FinishedAt string `json:"finishedAt"`
		} `json:"operationState"`
		Resources []struct {
			Group     string                   `json:"group"`
			Version   string                   `json:"version"`
			Kind      string                   `json:"kind"`
			Namespace string                   `json:"namespace"`
			Name      string                   `json:"name"`
			Status    string                   `json:"status"`
			Health    *struct{ Status string } `json:"health"`
		} `json:"resources"`
		History []struct {
			ID         int64  `json:"id"`
			Revision   string `json:"revision"`
			DeployedAt string `json:"deployedAt"`
			Source     *struct {
				RepoURL string `json:"repoURL"`
				Path    string `json:"path"`
			} `json:"source"`
		} `json:"history"`
	} `json:"status"`
}

func mapApp(raw argoApp) App {
	image := ""
	if len(raw.Status.Summary.Images) > 0 {
		image = raw.Status.Summary.Images[0]
	}

	lastSync := ""
	if raw.Status.OperationState != nil && raw.Status.OperationState.FinishedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.Status.OperationState.FinishedAt); err == nil {
			lastSync = fmtTimeAgo(t)
		}
	}

	var resources []AppResource
	for _, r := range raw.Status.Resources {
		health := ""
		if r.Health != nil {
			health = r.Health.Status
		}
		resources = append(resources, AppResource{
			Kind:      r.Kind,
			Name:      r.Name,
			Namespace: r.Namespace,
			Status:    r.Status,
			Health:    health,
			Group:     r.Group,
			Version:   r.Version,
		})
	}

	var history []AppHistoryEntry
	for _, h := range raw.Status.History {
		entry := AppHistoryEntry{
			ID:         h.ID,
			Revision:   h.Revision,
			DeployedAt: h.DeployedAt,
		}
		if h.Source != nil {
			entry.Source = h.Source.RepoURL
			if h.Source.Path != "" {
				entry.Source = h.Source.RepoURL + " · " + h.Source.Path
			}
		}
		history = append(history, entry)
	}
	// Newest first.
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	return App{
		Name:          raw.Metadata.Name,
		Namespace:     raw.Metadata.Namespace,
		Project:       raw.Spec.Project,
		SyncStatus:    raw.Status.Sync.Status,
		HealthStatus:  raw.Status.Health.Status,
		Image:         image,
		LastSync:      lastSync,
		RepoURL:       raw.Spec.Source.RepoURL,
		Path:          raw.Spec.Source.Path,
		TargetRev:     raw.Spec.Source.TargetRevision,
		DestServer:    raw.Spec.Destination.Server,
		DestNamespace: raw.Spec.Destination.Namespace,
		CreatedAt:     raw.Metadata.CreationTimestamp,
		Resources:     resources,
		History:       history,
	}
}

func fmtTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
