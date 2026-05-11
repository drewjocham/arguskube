package pkg

import (
	"fmt"

	"github.com/argues/argus/internal/argocd"
	"github.com/argues/argus/internal/k8s"
)

var errNoArgoCD = fmt.Errorf("Argo CD not configured — add URL and token in Settings")

// ArgusCDStatus returns whether ArgusCD (Argo CD) is connected.
type ArgusCDStatus struct {
	Connected bool   `json:"connected"`
	URL       string `json:"url"`
	Message   string `json:"message"`
}

// GetArgusCDStatus returns the current Argo CD connection state.
func (a *App) GetArgusCDStatus() ArgusCDStatus {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return ArgusCDStatus{
			Connected: false,
			Message:   "Argo CD not configured. Add URL and API token in Settings → ArgusCD.",
		}
	}
	// Test the connection.
	if err := a.argoCD.TestConnection(a.ctx); err != nil {
		return ArgusCDStatus{
			Connected: false,
			URL:       a.cfg.ArgoCD.URL,
			Message:   fmt.Sprintf("Connection failed: %s", err.Error()),
		}
	}
	return ArgusCDStatus{
		Connected: true,
		URL:       a.cfg.ArgoCD.URL,
		Message:   "Connected",
	}
}

// ListArgusCDApps returns all Argo CD applications. If Argo CD is not configured,
// falls back to listing Kubernetes deployments as applications.
func (a *App) ListArgusCDApps(project string) ([]argocd.App, error) {
	if a.argoCD != nil && a.argoCD.Connected() {
		return a.argoCD.ListApps(a.ctx, project)
	}

	// Fallback: list Kubernetes deployments as "applications".
	if a.k8s == nil {
		return nil, errNoCluster
	}
	k8sApps, err := a.k8s.ListApplications(a.ctx, "")
	if err != nil {
		return nil, err
	}
	return mapK8sAppsToArgoCD(k8sApps), nil
}

// GetArgusCDApp returns details for a single Argo CD application.
func (a *App) GetArgusCDApp(name string) (*argocd.App, error) {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return nil, errNoArgoCD
	}
	return a.argoCD.GetApp(a.ctx, name)
}

// GetArgusCDResources returns the resource tree for an Argo CD application.
func (a *App) GetArgusCDResources(name string) ([]argocd.AppResource, error) {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return nil, errNoArgoCD
	}
	return a.argoCD.GetAppResources(a.ctx, name)
}

// GetArgusCDDiffs returns drift/diff data for an Argo CD application.
func (a *App) GetArgusCDDiffs(name string) ([]argocd.AppDiff, error) {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return nil, errNoArgoCD
	}
	return a.argoCD.GetAppDiffs(a.ctx, name)
}

// SyncArgusCDApp triggers a sync. With Argo CD configured the call goes
// straight to the Argo CD API. Otherwise we fall back to restarting the
// matching Deployment in the Kubernetes API — which requires a namespace.
//
// `namespace` is optional. When empty (older callers / Argo CD path) we
// search across the cluster to find the Deployment by name. The k8s API
// errors out with "an empty namespace may not be set when a resource name
// is provided" if we hand it a name without a namespace, so we MUST resolve
// one here before delegating.
func (a *App) SyncArgusCDApp(name, namespace string) (*argocd.SyncResult, error) {
	if a.argoCD != nil && a.argoCD.Connected() {
		return a.argoCD.SyncApp(a.ctx, name)
	}
	if a.k8s == nil {
		return nil, errNoCluster
	}
	ns := namespace
	if ns == "" {
		resolved, err := a.k8s.FindDeploymentNamespace(a.ctx, name)
		if err != nil {
			return nil, fmt.Errorf("locate deployment %q for sync: %w", name, err)
		}
		ns = resolved
	}
	if err := a.k8s.RestartDeployment(a.ctx, ns, name); err != nil {
		return nil, err
	}
	return &argocd.SyncResult{Phase: "Succeeded", Message: fmt.Sprintf("Deployment %s/%s restarted", ns, name)}, nil
}

// RollbackArgusCDApp rolls back an Argo CD application to a previous revision.
func (a *App) RollbackArgusCDApp(name string, revisionID int64) error {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return errNoArgoCD
	}
	return a.argoCD.RollbackApp(a.ctx, name, revisionID)
}

// RefreshArgusCDApp refreshes an Argo CD application (fetches latest from Git).
func (a *App) RefreshArgusCDApp(name string, hard bool) error {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return errNoArgoCD
	}
	return a.argoCD.RefreshApp(a.ctx, name, hard)
}

// TestArgusCDConnection tests the Argo CD connection with the current settings.
func (a *App) TestArgusCDConnection() error {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return errNoArgoCD
	}
	return a.argoCD.TestConnection(a.ctx)
}

// ListArgusCDProjects returns the Argo CD AppProject names so the UI can
// populate a project filter. Returns an empty slice when Argo CD is not
// configured (the UI then hides the filter).
func (a *App) ListArgusCDProjects() ([]string, error) {
	if a.argoCD == nil || !a.argoCD.Connected() {
		return []string{}, nil
	}
	return a.argoCD.ListProjects(a.ctx)
}

// mapK8sAppsToArgoCD converts k8s.Application list to argocd.App for the fallback view.
func mapK8sAppsToArgoCD(apps []k8s.Application) []argocd.App {
	result := make([]argocd.App, 0, len(apps))
	for _, a := range apps {
		result = append(result, argocd.App{
			Name:          a.Name,
			Namespace:     a.Namespace,
			Project:       "default",
			SyncStatus:    a.SyncStatus,
			HealthStatus:  a.HealthStatus,
			Replicas:      a.Replicas,
			ReadyReplicas: a.ReadyReplicas,
			Image:         a.Image,
			LastSync:      a.LastSync,
			DestServer:    "https://kubernetes.default.svc",
			DestNamespace: a.Namespace,
		})
	}
	return result
}
