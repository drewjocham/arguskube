package k8s

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigyaml "sigs.k8s.io/yaml"
)

type MigrationResult struct {
	OriginalIngress string `json:"originalIngress"`
	GatewayYAML     string `json:"gatewayYAML"`
	HTTPRouteYAML   string `json:"httpRouteYAML"`
	Warnings        []string `json:"warnings,omitempty"`
}

func TranslateIngressToGateway(ingressYAML string) (*MigrationResult, error) {
	ingress := &networkingv1.Ingress{}
	if err := sigyaml.Unmarshal([]byte(ingressYAML), ingress); err != nil {
		return nil, fmt.Errorf("parse ingress: %w", err)
	}

	result := &MigrationResult{
		OriginalIngress: ingressYAML,
	}

	name := ingress.Name
	if name == "" {
		name = "migrated"
	}
	namespace := ingress.Namespace
	if namespace == "" {
		namespace = "default"
	}
	gwClassName := ""
	if ingress.Spec.IngressClassName != nil {
		gwClassName = *ingress.Spec.IngressClassName
	}

	listeners := buildGatewayListeners(ingress)
	gateway := buildGatewayObject(name, namespace, gwClassName, listeners)
	gwYAML, err := yamlMarshalUnstructured(gateway)
	if err != nil {
		return nil, fmt.Errorf("marshal gateway: %w", err)
	}
	result.GatewayYAML = string(gwYAML)

	route := buildHTTPRouteFromIngress(name, namespace, ingress, gwClassName)
	routeYAML, err := yamlMarshalUnstructured(route)
	if err != nil {
		return nil, fmt.Errorf("marshal httproute: %w", err)
	}
	result.HTTPRouteYAML = string(routeYAML)

	if ingress.Spec.DefaultBackend != nil {
		result.Warnings = append(result.Warnings, "Default backend converted to catch-all route rule")
	}

	return result, nil
}

func buildGatewayListeners(ingress *networkingv1.Ingress) []map[string]interface{} {
	seenHosts := make(map[string]bool)
	var listeners []map[string]interface{}

	for _, tls := range ingress.Spec.TLS {
		host := ""
		if len(tls.Hosts) > 0 {
			host = tls.Hosts[0]
		}
		if seenHosts[host] {
			continue
		}
		seenHosts[host] = true

		listener := map[string]interface{}{
			"name":     fmt.Sprintf("https-%s", orDefault(host, "default")),
			"port":     int64(443),
			"protocol": "HTTPS",
			"hostname": host,
			"tls": map[string]interface{}{
				"mode": "Terminate",
				"certificateRefs": []map[string]interface{}{
					{"name": tls.SecretName},
				},
			},
		}
		if host == "" {
			delete(listener, "hostname")
		}
		listeners = append(listeners, listener)
	}

	if len(listeners) == 0 {
		listener := map[string]interface{}{
			"name":     "http",
			"port":     int64(80),
			"protocol": "HTTP",
		}
		listeners = append(listeners, listener)
	}

	return listeners
}

func buildGatewayObject(name, namespace, className string, listeners []map[string]interface{}) *unstructured.Unstructured {
	gwLabels := map[string]interface{}{
		"app.kubernetes.io/managed-by": "argus-migration",
	}
	if className == "" {
		className = "istio" // default assumption
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    gwLabels,
			},
			"spec": map[string]interface{}{
				"gatewayClassName": className,
				"listeners":        listeners,
			},
		},
	}
}

func buildHTTPRouteFromIngress(name, namespace string, ingress *networkingv1.Ingress, gwClassName string) *unstructured.Unstructured {
	rules := make([]map[string]interface{}, 0)

	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		if rule.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			backendName := path.Backend.Service.Name
			backendPort := int64(80)
			if path.Backend.Service.Port.Number > 0 {
				backendPort = int64(path.Backend.Service.Port.Number)
			} else if path.Backend.Service.Port.Name != "" {
				backendPort = 80
			}

			pathType := "PathPrefix"
			if path.PathType != nil {
				switch *path.PathType {
				case networkingv1.PathTypeExact:
					pathType = "Exact"
				case networkingv1.PathTypePrefix:
					pathType = "PathPrefix"
				default:
					pathType = "PathPrefix"
				}
			}

			rule := map[string]interface{}{
				"matches": []map[string]interface{}{
					{
						"path": map[string]interface{}{
							"type":  pathType,
							"value": path.Path,
						},
					},
				},
				"backendRefs": []map[string]interface{}{
					{
						"name": backendName,
						"port": backendPort,
						"weight": int64(1),
					},
				},
			}

			if host != "" {
				rule["hostnames"] = []string{host}
			}

			rules = append(rules, rule)
		}
	}

	parentRefs := []map[string]interface{}{
		{
			"name":      name,
			"namespace": namespace,
		},
	}

	route := map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "argus-migration",
			},
		},
		"spec": map[string]interface{}{
			"parentRefs": parentRefs,
			"rules":      rules,
		},
	}

	// Collect hostnames
	var hostnames []string
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hostnames = append(hostnames, rule.Host)
		}
	}
	if len(hostnames) > 0 {
		route["spec"].(map[string]interface{})["hostnames"] = hostnames
	}

	return &unstructured.Unstructured{Object: route}
}

func yamlMarshalUnstructured(obj *unstructured.Unstructured) ([]byte, error) {
	jsonBytes, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return sigyaml.JSONToYAML(jsonBytes)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return strings.ReplaceAll(s, ".", "-")
}
