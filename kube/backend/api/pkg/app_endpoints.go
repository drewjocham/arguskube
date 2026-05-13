package pkg

import (
	"github.com/argues/argus/internal/k8s"
)

// See app_authjuggler.go for why ctx is dropped from Wails bindings —
// the runtime doesn't auto-inject it, so the frontend's arg count
// always trips the "received N, expected N+1" check. Use a.appCtx().

func (a *App) AnalyzeEndpointReadiness(namespace, serviceName string) (*k8s.EndpointReadiness, error) {
	return a.k8s.AnalyzeEndpointReadiness(a.appCtx(), namespace, serviceName)
}

func (a *App) ListExternalBridges(namespace string) ([]k8s.ExternalBridge, error) {
	return a.k8s.ListExternalBridges(a.appCtx(), namespace)
}

func (a *App) CreateExternalBridge(spec *k8s.ExternalBridgeSpec) (*k8s.BridgeResult, error) {
	return a.k8s.CreateExternalBridge(a.appCtx(), spec)
}

func (a *App) PingExternalEndpoint(namespace, name string) (bool, error) {
	return a.k8s.PingExternalEndpoint(a.appCtx(), namespace, name)
}

func (a *App) AnalyzeLabelMatch(namespace, serviceName string) (*k8s.LabelDiffResult, error) {
	return a.k8s.AnalyzeLabelMatch(a.appCtx(), namespace, serviceName)
}

func (a *App) FindOrphanedEndpoints(namespace string) ([]k8s.OrphanedEndpoint, error) {
	return a.k8s.FindOrphanedEndpoints(a.appCtx(), namespace)
}

func (a *App) ListServiceSelectors(namespace string) ([]k8s.ServiceSelectorInfo, error) {
	return a.k8s.ListServiceSelectors(a.appCtx(), namespace)
}

func (a *App) ListEndpointSlices(namespace string) ([]k8s.EndpointSliceInfo, error) {
	return a.k8s.ListEndpointSlices(a.appCtx(), namespace)
}

func (a *App) GetZoneDistribution(namespace, serviceName string) (*k8s.ZoneDistribution, error) {
	return a.k8s.GetZoneDistribution(a.appCtx(), namespace, serviceName)
}

func (a *App) CheckTopologyAwareRouting(namespace string) ([]k8s.TopologyWarning, error) {
	return a.k8s.CheckTopologyAwareRouting(a.appCtx(), namespace)
}
