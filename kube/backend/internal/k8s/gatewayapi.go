package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	sigyaml "sigs.k8s.io/yaml"
)

var gatewayGVR = map[string]schema.GroupVersionResource{
	"gatewayclasses":  {Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"},
	"gateways":        {Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"},
	"httproutes":      {Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"},
	"referencegrants": {Group: "gateway.networking.k8s.io", Version: "v1", Resource: "referencegrants"},
}

type GatewaySummary struct {
	Name           string             `json:"name"`
	Namespace      string             `json:"namespace"`
	ClassName      string             `json:"className"`
	Listeners      int                `json:"listeners"`
	Addresses      []string           `json:"addresses"`
	Conditions     []GatewayCondition `json:"conditions"`
	AttachedRoutes int                `json:"attachedRoutes"`
}

type GatewayCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type HTTPRouteSummary struct {
	Name        string              `json:"name"`
	Namespace   string              `json:"namespace"`
	Hostnames   []string            `json:"hostnames"`
	ParentRefs  []RouteParentRef    `json:"parentRefs"`
	Conditions  []GatewayCondition  `json:"conditions"`
	Matches     int                 `json:"matches"`
	BackendRefs []BackendRefSummary `json:"backendRefs"`
}

type RouteParentRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Group     string `json:"group"`
	Kind      string `json:"kind"`
}

type BackendRefSummary struct {
	Name   string `json:"name"`
	Port   int64  `json:"port"`
	Weight int64  `json:"weight"`
}

type GatewayRouteGraph struct {
	Gateways   []GatewaySummary  `json:"gateways"`
	HTTPRoutes []HTTPRouteSummary `json:"httpRoutes"`
	Conflicts  []RouteConflict   `json:"conflicts,omitempty"`
}

type RouteConflict struct {
	Hostname   string `json:"hostname"`
	NamespaceA string `json:"namespaceA"`
	RouteNameA string `json:"routeNameA"`
	NamespaceB string `json:"namespaceB"`
	RouteNameB string `json:"routeNameB"`
}

func (c *Client) dynClient() (dynamic.Interface, error) {
	return dynamic.NewForConfig(c.restCfg)
}

func (c *Client) ListGateways(ctx context.Context, namespace string) ([]GatewaySummary, error) {
	dyn, err := c.dynClient()
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	var list *unstructured.UnstructuredList
	if namespace != "" {
		list, err = dyn.Resource(gatewayGVR["gateways"]).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = dyn.Resource(gatewayGVR["gateways"]).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("list gateways: %w", err)
	}

	gateways := make([]GatewaySummary, 0, len(list.Items))
	for _, obj := range list.Items {
		gateways = append(gateways, gatewayFromUnstructured(obj))
	}
	return gateways, nil
}

func (c *Client) ListGatewayClasses(ctx context.Context) ([]GatewaySummary, error) {
	dyn, err := c.dynClient()
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	list, err := dyn.Resource(gatewayGVR["gatewayclasses"]).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list gatewayclasses: %w", err)
	}

	classes := make([]GatewaySummary, 0, len(list.Items))
	for _, obj := range list.Items {
		gw := GatewaySummary{
			Name: obj.GetName(),
		}
		params, _, _ := unstructured.NestedString(obj.Object, "spec", "parametersRef", "name")
		if params != "" {
			gw.ClassName = params
		}
		conds, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		for _, c := range conds {
			if cm, ok := c.(map[string]interface{}); ok {
				gw.Conditions = append(gw.Conditions, GatewayCondition{
					Type:    stringOr(cm, "type"),
					Status:  stringOr(cm, "status"),
					Reason:  stringOr(cm, "reason"),
					Message: stringOr(cm, "message"),
				})
			}
		}
		classes = append(classes, gw)
	}
	return classes, nil
}

func (c *Client) ListHTTPRoutes(ctx context.Context, namespace string) ([]HTTPRouteSummary, error) {
	dyn, err := c.dynClient()
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	var list *unstructured.UnstructuredList
	if namespace != "" {
		list, err = dyn.Resource(gatewayGVR["httproutes"]).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = dyn.Resource(gatewayGVR["httproutes"]).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("list httproutes: %w", err)
	}

	routes := make([]HTTPRouteSummary, 0, len(list.Items))
	for _, obj := range list.Items {
		routes = append(routes, httpRouteFromUnstructured(obj))
	}
	return routes, nil
}

func (c *Client) GetRouteTopologyGraph(ctx context.Context, namespace string) (*GatewayRouteGraph, error) {
	gateways, err := c.ListGateways(ctx, namespace)
	if err != nil {
		return nil, err
	}

	routes, err := c.ListHTTPRoutes(ctx, "")
	if err != nil {
		return nil, err
	}

	graph := &GatewayRouteGraph{
		Gateways:   gateways,
		HTTPRoutes: routes,
	}

	hostnameRoutes := make(map[string][]string)
	hostnameNamespaces := make(map[string][]string)
	for _, r := range routes {
		for _, h := range r.Hostnames {
			hostnameRoutes[h] = append(hostnameRoutes[h], r.Name)
			hostnameNamespaces[h] = append(hostnameNamespaces[h], r.Namespace)
		}
	}
	for hostname, routeNames := range hostnameRoutes {
		if len(routeNames) > 1 {
			graph.Conflicts = append(graph.Conflicts, RouteConflict{
				Hostname:   hostname,
				RouteNameA: routeNames[0],
				NamespaceA: hostnameNamespaces[hostname][0],
				RouteNameB: routeNames[1],
				NamespaceB: hostnameNamespaces[hostname][1],
			})
		}
	}

	return graph, nil
}

func (c *Client) GetGatewayStatusByRole(ctx context.Context, role string) (interface{}, error) {
	switch role {
	case "operator":
		classes, err := c.ListGatewayClasses(ctx)
		if err != nil {
			return nil, err
		}
		gateways, err := c.ListGateways(ctx, "")
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"gatewayClasses": classes,
			"gateways":       gateways,
		}, nil
	case "developer":
		routes, err := c.ListHTTPRoutes(ctx, "")
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"httpRoutes": routes,
		}, nil
	default:
		graph, err := c.GetRouteTopologyGraph(ctx, "")
		if err != nil {
			return nil, err
		}
		return graph, nil
	}
}

func gatewayFromUnstructured(obj unstructured.Unstructured) GatewaySummary {
	gw := GatewaySummary{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	gwcName, _, _ := unstructured.NestedString(obj.Object, "spec", "gatewayClassName")
	gw.ClassName = gwcName

	listeners, _, _ := unstructured.NestedSlice(obj.Object, "spec", "listeners")
	gw.Listeners = len(listeners)

	addrs, _, _ := unstructured.NestedSlice(obj.Object, "status", "addresses")
	for _, a := range addrs {
		if am, ok := a.(map[string]interface{}); ok {
			if v, ok2 := am["value"]; ok2 {
				gw.Addresses = append(gw.Addresses, fmt.Sprintf("%v", v))
			}
		}
	}

	conds, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, c := range conds {
		if cm, ok := c.(map[string]interface{}); ok {
			gw.Conditions = append(gw.Conditions, GatewayCondition{
				Type:    stringOr(cm, "type"),
				Status:  stringOr(cm, "status"),
				Reason:  stringOr(cm, "reason"),
				Message: stringOr(cm, "message"),
			})
		}
	}

	listenerStatuses, _, _ := unstructured.NestedSlice(obj.Object, "status", "listeners")
	for _, ls := range listenerStatuses {
		if lsm, ok := ls.(map[string]interface{}); ok {
			statusAttached, _, _ := unstructured.NestedInt64(lsm, "attachedRoutes")
			gw.AttachedRoutes += int(statusAttached)
		}
	}

	return gw
}

func httpRouteFromUnstructured(obj unstructured.Unstructured) HTTPRouteSummary {
	route := HTTPRouteSummary{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	hostnames, _, _ := unstructured.NestedStringSlice(obj.Object, "spec", "hostnames")
	route.Hostnames = hostnames

	parentRefs, _, _ := unstructured.NestedSlice(obj.Object, "spec", "parentRefs")
	for _, pr := range parentRefs {
		if prm, ok := pr.(map[string]interface{}); ok {
			ref := RouteParentRef{
				Name: stringOr(prm, "name"),
				Group: stringOrDefault(prm, "group", "gateway.networking.k8s.io"),
				Kind:  stringOrDefault(prm, "kind", "Gateway"),
			}
			if ns, ok2 := prm["namespace"]; ok2 {
				ref.Namespace = fmt.Sprintf("%v", ns)
			}
			route.ParentRefs = append(route.ParentRefs, ref)
		}
	}

	rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	route.Matches = len(rules)
	for _, rule := range rules {
		if rulem, ok := rule.(map[string]interface{}); ok {
			backends, _, _ := unstructured.NestedSlice(rulem, "backendRefs")
			for _, br := range backends {
				if brm, ok := br.(map[string]interface{}); ok {
					ref := BackendRefSummary{
						Name:   stringOr(brm, "name"),
						Weight: int64OrDefault(brm, "weight", 1),
						Port:   int64OrZero(brm, "port"),
					}
					route.BackendRefs = append(route.BackendRefs, ref)
				}
			}
		}
	}

	parents, _, _ := unstructured.NestedSlice(obj.Object, "status", "parents")
	for _, p := range parents {
		if pm, ok := p.(map[string]interface{}); ok {
			pConds, _, _ := unstructured.NestedSlice(pm, "conditions")
			for _, c := range pConds {
				if cm, ok := c.(map[string]interface{}); ok {
					route.Conditions = append(route.Conditions, GatewayCondition{
						Type:    stringOr(cm, "type"),
						Status:  stringOr(cm, "status"),
						Reason:  stringOr(cm, "reason"),
						Message: stringOr(cm, "message"),
					})
				}
			}
		}
	}

	return route
}

func stringOr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func stringOrDefault(m map[string]interface{}, key, def string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return def
}

func int64OrZero(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		i, _ := toInt64(v)
		return i
	}
	return 0
}

func int64OrDefault(m map[string]interface{}, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		i, err := toInt64(v)
		if err == nil {
			return i
		}
	}
	return def
}

func toInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case int:
		return int64(val), nil
	case string:
		var i int64
		_, err := fmt.Sscanf(val, "%d", &i)
		return i, err
	}
	return 0, fmt.Errorf("cannot convert %T to int64", v)
}

var _ = sigyaml.JSONToYAML

type BackendRefWeight struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Port      int64  `json:"port"`
	Weight    int64  `json:"weight"`
}

func (c *Client) GenerateTrafficSplitHTTPRoute(ctx context.Context, routeName, namespace, gatewayName, gatewayNamespace string, backends []BackendRefWeight) (string, error) {
	if len(backends) == 0 {
		return "", fmt.Errorf("at least one backend is required")
	}

	backendRefs := make([]map[string]interface{}, 0, len(backends))
	for _, b := range backends {
		if b.Weight <= 0 {
			b.Weight = 1
		}
		ref := map[string]interface{}{
			"name":   b.Name,
			"port":   b.Port,
			"weight": b.Weight,
		}
		if b.Namespace != "" {
			ref["namespace"] = b.Namespace
		}
		backendRefs = append(backendRefs, ref)
	}

	route := map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name":      routeName,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "argus-gateway",
			},
		},
		"spec": map[string]interface{}{
			"parentRefs": []map[string]interface{}{
				{
					"name":      gatewayName,
					"namespace": gatewayNamespace,
				},
			},
			"rules": []map[string]interface{}{
				{
					"backendRefs": backendRefs,
				},
			},
		},
	}

	obj := &unstructured.Unstructured{Object: route}
	jsonBytes, err := obj.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}

	yamlBytes, err := sigyaml.JSONToYAML(jsonBytes)
	if err != nil {
		return "", fmt.Errorf("json to yaml: %w", err)
	}

	return string(yamlBytes), nil
}
