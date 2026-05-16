package pkg

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/k8s"
)

type K8sHandler struct {
	app *App
}

func NewK8sHandler(app *App) *K8sHandler {
	return &K8sHandler{app: app}
}

func (h *K8sHandler) GetClusterInfo() (*k8s.ClusterInfo, error) {
	if h.app.k8s == nil {
		return &k8s.ClusterInfo{Name: "not connected", NodeCount: 0, K8sVersion: "—"}, nil
	}
	return h.app.k8s.GetClusterInfo(h.app.ctx)
}

func (h *K8sHandler) ListContexts() ([]k8s.ContextInfo, error) {
	if h.app.k8s != nil {
		return h.app.k8s.ListContexts()
	}
	kubeconfigPath := ""
	if h.app.cfg != nil {
		kubeconfigPath = h.app.cfg.Kubernetes.Config
	}
	return k8s.ListContextsFromKubeconfig(kubeconfigPath, "")
}

func (h *K8sHandler) AutoResolveContext() (k8s.ContextResolution, error) {
	kubeconfigPath := ""
	activeOverride := ""
	if h.app.cfg != nil {
		kubeconfigPath = h.app.cfg.Kubernetes.Config
		activeOverride = h.app.cfg.Kubernetes.Context
	}

	emitK8sStatus(h.app, "info", "Scanning kubeconfig for contexts…", "")

	probes, err := k8s.ProbeContexts(h.app.ctx, kubeconfigPath, activeOverride, 2*time.Second)
	if err != nil {
		emitK8sStatus(h.app, "error", "Could not read kubeconfig", err.Error())
		return k8s.ContextResolution{}, err
	}
	if len(probes) == 0 {
		emitK8sStatus(h.app, "warn", "No kubeconfig contexts found",
			"Add a context with kubectl or via the settings checklist.")
		return k8s.ContextResolution{Confidence: "none", Probes: probes}, k8s.ErrNoContexts
	}

	for _, p := range probes {
		if p.Reachable {
			emitK8sStatus(h.app, "info",
				fmt.Sprintf("%s reachable (%dms, %s)", p.Name, p.LatencyMs, p.ServerVersion), "")
		} else {
			emitK8sStatus(h.app, "warn",
				fmt.Sprintf("%s unreachable", p.Name), p.Error)
		}
	}

	res := k8s.ChooseContext(probes)
	if res.Chosen == "" {
		emitK8sStatus(h.app, "warn", "No reachable contexts",
			"Argus will retry on the next manual switch.")
		return res, nil
	}

	if err := h.app.SwitchContext(res.Chosen); err != nil {
		emitK8sStatus(h.app, "error",
			fmt.Sprintf("Could not switch to %s", res.Chosen), err.Error())
		return res, err
	}

	switch res.Confidence {
	case "active-reachable":
		emitK8sStatus(h.app, "info", fmt.Sprintf("Connected to %s", res.Chosen), "")
	case "fallback-reachable":
		emitK8sStatus(h.app, "warn",
			fmt.Sprintf("Active context unreachable — using %s", res.Chosen),
			"You can switch back via the sidebar context picker.")
	case "active-unreachable":
		emitK8sStatus(h.app, "warn",
			fmt.Sprintf("%s is selected but unreachable", res.Chosen),
			"Argus will keep retrying. Common cause: VPN off or corporate proxy.")
	}
	return res, nil
}

func (h *K8sHandler) SwitchContext(name string) error {
	return h.app.SwitchContext(name)
}

func (h *K8sHandler) GetMetrics() (*alerts.ClusterMetrics, error) {
	if h.app.k8s == nil {
		return &alerts.ClusterMetrics{SLOStatus: "unknown"}, nil
	}
	m, err := h.app.k8s.GetMetrics(h.app.ctx)
	if err == nil && m != nil {
		h.app.cachedMetrics.Store(m)
	}
	return m, err
}

func (h *K8sHandler) GetDeploymentRevisions(namespace, deployment string, limit int) ([]k8s.DeploymentRevision, error) {
	if h.app.k8s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	return h.app.k8s.GetDeploymentRevisions(h.app.ctx, namespace, deployment, limit)
}

func (h *K8sHandler) GetVPARecommendations(namespace string) ([]k8s.VPARecommendation, error) {
	if h.app.k8s == nil {
		return nil, nil
	}
	return h.app.k8s.GetVPARecommendations(h.app.ctx, namespace)
}

func (h *K8sHandler) GetServicePods(namespace, serviceName string) ([]k8s.ServicePod, error) {
	if h.app.k8s == nil {
		return nil, nil
	}
	return h.app.k8s.GetServicePods(h.app.ctx, namespace, serviceName)
}

func emitK8sStatus(a *App, severity, message, detail string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, EventArgusStatus, map[string]any{
		"source":   "k8s",
		"severity": severity,
		"message":  message,
		"detail":   detail,
	})
}
