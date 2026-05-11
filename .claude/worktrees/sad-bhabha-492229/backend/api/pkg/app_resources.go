package pkg

import (
	"fmt"
	"log/slog"

	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/features"
	"github.com/argues/kube-watcher/internal/k8s"
	"github.com/argues/kube-watcher/internal/vulnscan"
)

// ListResources lists resources of the given kind in a namespace.
func (a *App) ListResources(kind, namespace string) (*k8s.ResourceListResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.ListResources(a.ctx, kind, namespace)
}

// GetResourceDetail returns full details for a specific resource.
func (a *App) GetResourceDetail(kind, namespace, name string) (*k8s.ResourceDetailResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.GetResourceDetail(a.ctx, kind, namespace, name)
}

// ListAllNamespaces returns all namespace names for the namespace picker.
func (a *App) ListAllNamespaces() ([]string, error) {
	if a.k8s == nil {
		return nil, nil
	}
	return a.k8s.ListAllNamespaces(a.ctx)
}

// GetTopology builds a topology graph from the live cluster state.
func (a *App) GetTopology(namespace string) (*k8s.TopologyResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.BuildTopology(a.ctx, namespace)
}

// ListApplications returns deployment-based "applications" with rollout status.
func (a *App) ListApplications(namespace string) ([]k8s.Application, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.ListApplications(a.ctx, namespace)
}

// SyncApplication triggers a rollout restart on a deployment.
func (a *App) SyncApplication(namespace, name string) error {
	if a.k8s == nil {
		return errNoCluster
	}
	return a.k8s.RestartDeployment(a.ctx, namespace, name)
}

// QueryLogs searches pod logs across the cluster with text filter.
func (a *App) QueryLogs(query, namespace string, limit int) (*k8s.LogQueryResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.QueryLogs(a.ctx, query, namespace, limit)
}

// DeletePod deletes a pod by namespace and name.
func (a *App) DeletePod(namespace, podName string) error {
	if a.k8s == nil {
		return errNoCluster
	}
	a.logger.Info("deleting pod", slog.String("namespace", namespace), slog.String("pod", podName))
	return a.k8s.DeletePod(a.ctx, namespace, podName)
}

// QueryTimeSeriesMetrics returns time-series data points for a given query.
// Queries real metrics-server if available, falls back to core API derivation.
func (a *App) QueryTimeSeriesMetrics(query string, timeRange string) ([]float64, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.QueryTimeSeriesMetrics(a.ctx, query, timeRange)
}

// GetAnomalyJobs returns the configured Anomstack jobs.
func (a *App) GetAnomalyJobs() ([]anomaly.Job, error) {
	if !a.gate.Allowed(features.FeatureAnomstack) {
		return nil, features.ErrProRequired
	}
	if a.detector == nil {
		return nil, nil
	}
	return a.detector.ListJobs(a.ctx)
}

// GetAnomalySettings returns the saved anomaly detection configuration.
func (a *App) GetAnomalySettings() (anomaly.Settings, error) {
	if a.anomalySettings == nil {
		return anomaly.Settings{
			Sensitivity: 30, BaselineWindow: 7, MetricType: "cpu",
			Algorithm: "smart", Threshold: 85, TargetScope: "all",
		}, nil
	}
	return a.anomalySettings.GetSettings(), nil
}

// SaveAnomalySettings persists anomaly detection configuration.
func (a *App) SaveAnomalySettings(settings anomaly.Settings) error {
	if a.anomalySettings == nil {
		return fmt.Errorf("anomaly settings store not initialized")
	}
	return a.anomalySettings.SaveSettings(settings)
}

// GetAnomalyRules returns all anomaly detection rules.
func (a *App) GetAnomalyRules() ([]anomaly.Rule, error) {
	if a.anomalySettings == nil {
		return nil, nil
	}
	return a.anomalySettings.ListRules(), nil
}

// SaveAnomalyRule creates or updates an anomaly detection rule.
func (a *App) SaveAnomalyRule(rule anomaly.Rule) error {
	if a.anomalySettings == nil {
		return fmt.Errorf("anomaly settings store not initialized")
	}
	return a.anomalySettings.SaveRule(rule)
}

// ToggleAnomalyRule flips the enabled state of a rule and returns the new state.
func (a *App) ToggleAnomalyRule(id string) (bool, error) {
	if a.anomalySettings == nil {
		return false, fmt.Errorf("anomaly settings store not initialized")
	}
	return a.anomalySettings.ToggleRule(id)
}

// DeleteAnomalyRule removes a rule by ID.
func (a *App) DeleteAnomalyRule(id string) error {
	if a.anomalySettings == nil {
		return fmt.Errorf("anomaly settings store not initialized")
	}
	return a.anomalySettings.DeleteRule(id)
}

// ListVulnerabilities returns cached scan results (or demo data if no scan has run).
func (a *App) ListVulnerabilities() ([]vulnscan.ScannedImage, error) {
	if a.scanner == nil {
		return vulnscan.DemoResults(), nil
	}
	return a.scanner.List(), nil
}

// ScanImage triggers a Trivy vulnerability scan for a single container image.
func (a *App) ScanImage(image string, engine string) (string, error) {
	if a.scanner == nil {
		return "Scanner not initialized — no cluster connection", nil
	}
	return a.scanner.ScanSingleImage(a.ctx, image, engine)
}

// ScanAllImages enumerates all images in the cluster and scans each via Trivy.
func (a *App) ScanAllImages(namespace string) ([]vulnscan.ScannedImage, error) {
	if a.scanner == nil {
		return vulnscan.DemoResults(), nil
	}
	if namespace == "" {
		namespace = ""
	}
	return a.scanner.ScanAll(a.ctx, namespace)
}

// ScaleDeployment scales a deployment to a new replica count (stub for future use).
// Currently not exposed but kept for planned operations API.
func (a *App) ScaleDeployment(namespace, name string, replicas int32) error {
	if a.k8s == nil {
		return errNoCluster
	}
	return a.k8s.ScaleDeployment(a.ctx, namespace, name, replicas)
}

// RestartDeployment restarts a deployment by updating the pod template spec (stub).
// Currently not exposed but kept for planned operations API.
func (a *App) RestartDeployment(namespace, name string) error {
	if a.k8s == nil {
		return errNoCluster
	}
	return a.k8s.RestartDeployment(a.ctx, namespace, name)
}

// EstimateCosts returns a FinOps cost report based on pod resource requests.
// provider is one of "aws", "gcp", "azure", "digitalocean". Empty defaults to AWS.
func (a *App) EstimateCosts(provider string) (*k8s.ClusterCostReport, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	cfg := k8s.CostConfigForProvider(k8s.CloudProvider(provider))
	return a.k8s.EstimateCosts(a.ctx, cfg)
}
