package pkg

import (
	"github.com/argues/argus/internal/k8s"
)

// See app_authjuggler.go: ctx is dropped from Wails bindings; use a.appCtx().

func (a *App) ListGateways(namespace string) ([]k8s.GatewaySummary, error) {
	return a.k8s.ListGateways(a.appCtx(), namespace)
}

func (a *App) ListGatewayClasses() ([]k8s.GatewaySummary, error) {
	return a.k8s.ListGatewayClasses(a.appCtx())
}

func (a *App) ListHTTPRoutes(namespace string) ([]k8s.HTTPRouteSummary, error) {
	return a.k8s.ListHTTPRoutes(a.appCtx(), namespace)
}

func (a *App) GetRouteTopologyGraph(namespace string) (*k8s.GatewayRouteGraph, error) {
	return a.k8s.GetRouteTopologyGraph(a.appCtx(), namespace)
}

func (a *App) GetGatewayStatusByRole(role string) (interface{}, error) {
	return a.k8s.GetGatewayStatusByRole(a.appCtx(), role)
}

func (a *App) TranslateIngressToGateway(ingressYAML string) (*k8s.MigrationResult, error) {
	return k8s.TranslateIngressToGateway(ingressYAML)
}

func (a *App) GenerateTrafficSplitHTTPRoute(routeName, namespace, gatewayName, gatewayNamespace string, backends []k8s.BackendRefWeight) (string, error) {
	return a.k8s.GenerateTrafficSplitHTTPRoute(a.appCtx(), routeName, namespace, gatewayName, gatewayNamespace, backends)
}
