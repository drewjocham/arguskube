package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// resourceGVR maps a plural resource kind to its GroupVersionResource.
// Add entries here as new resource types are needed.
var resourceGVR = map[string]schema.GroupVersionResource{
	"deployments":     {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulsets":    {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonsets":      {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"replicasets":     {Group: "apps", Version: "v1", Resource: "replicasets"},
	"pods":            {Group: "", Version: "v1", Resource: "pods"},
	"services":        {Group: "", Version: "v1", Resource: "services"},
	"configmaps":      {Group: "", Version: "v1", Resource: "configmaps"},
	"secrets":         {Group: "", Version: "v1", Resource: "secrets"},
	"namespaces":      {Group: "", Version: "v1", Resource: "namespaces"},
	"nodes":           {Group: "", Version: "v1", Resource: "nodes"},
	"pvcs":            {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"pv":              {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"ingresses":       {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"networkpolicies": {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	"jobs":            {Group: "batch", Version: "v1", Resource: "jobs"},
	"cronjobs":        {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"hpas":            {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
}

// GetResourceYaml returns the raw YAML manifest of any Kubernetes resource
// by fetching it through the dynamic client.
func (c *Client) GetResourceYaml(ctx context.Context, kind, namespace, name string) (string, error) {
	gvr, ok := resourceGVR[kind]
	if !ok {
		return "", fmt.Errorf("unsupported resource kind: %s", kind)
	}

	dynClient, err := dynamic.NewForConfig(c.restCfg)
	if err != nil {
		return "", fmt.Errorf("dynamic client: %w", err)
	}

	var obj *unstructured.Unstructured
	if namespace != "" && gvr.Group == "" && gvr.Resource == "namespaces" {
		// Namespace-scoped, but namespaces are cluster-scoped.
		obj, err = dynClient.Resource(gvr).Get(ctx, name, getOpts)
	} else if namespace != "" && kind != "nodes" && kind != "namespaces" && kind != "pv" {
		obj, err = dynClient.Resource(gvr).Namespace(namespace).Get(ctx, name, getOpts)
	} else {
		obj, err = dynClient.Resource(gvr).Get(ctx, name, getOpts)
	}
	if err != nil {
		return "", fmt.Errorf("get %s/%s: %w", kind, name, err)
	}

	// Strip resourceVersion, uid, etc. to produce a clean "apply-able" manifest.
	cleanForRedeploy(obj)

	rawJSON, err := obj.MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}

	yamlBytes, err := yaml.JSONToYAML(rawJSON)
	if err != nil {
		return "", fmt.Errorf("json to yaml: %w", err)
	}

	return string(yamlBytes), nil
}

// ApplyYaml applies a raw YAML manifest to the cluster via the dynamic client.
// It attempts a create-first, fallback-to-update strategy.
func (c *Client) ApplyYaml(ctx context.Context, yamlContent string) (string, error) {
	// Convert YAML to unstructured object.
	jsonBytes, err := yaml.YAMLToJSON([]byte(yamlContent))
	if err != nil {
		return "", fmt.Errorf("yaml to json: %w", err)
	}

	obj := &unstructured.Unstructured{}
	if _, _, err := unstructured.UnstructuredJSONScheme.Decode(jsonBytes, nil, obj); err != nil {
		// If scheme decoding fails, try Unmarshal directly.
		if err2 := obj.UnmarshalJSON(jsonBytes); err2 != nil {
			return "", fmt.Errorf("parse manifest: yaml->json: %w, direct: %v", err, err2)
		}
	}

	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resourceNameForGVK(gvk),
	}
	if gvr.Resource == "" {
		return "", fmt.Errorf("cannot determine resource from kind: %s", gvk.Kind)
	}

	dynClient, err := dynamic.NewForConfig(c.restCfg)
	if err != nil {
		return "", fmt.Errorf("dynamic client: %w", err)
	}

	ns := obj.GetNamespace()
	name := obj.GetName()

	// Remove fields that prevent re-creation.
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetSelfLink("")
	obj.SetCreationTimestamp(unstructured.Unstructured{}.GetCreationTimestamp())
	obj.SetManagedFields(nil)

	// Try create first.
	_, err = dynClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1CreateOpts)
	if err == nil {
		return fmt.Sprintf("Created %s/%s", gvk.Kind, name), nil
	}

	// If conflict or already-exists, fall back to update.
	if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "conflict") {
		// Get the current resource to obtain the latest resourceVersion.
		current, getErr := dynClient.Resource(gvr).Namespace(ns).Get(ctx, name, getOpts)
		if getErr != nil {
			return "", fmt.Errorf("create failed, then get existing: %w (initial: %v)", getErr, err)
		}
		obj.SetResourceVersion(current.GetResourceVersion())
		_, updateErr := dynClient.Resource(gvr).Namespace(ns).Update(ctx, obj, metav1UpdateOpts)
		if updateErr != nil {
			return "", fmt.Errorf("create failed (%v), then update: %w", err, updateErr)
		}
		return fmt.Sprintf("Updated %s/%s", gvk.Kind, name), nil
	}

	return "", fmt.Errorf("create %s: %w", gvk.Kind, err)
}

// DeleteResource deletes a single resource by kind, namespace, and name.
func (c *Client) DeleteResource(ctx context.Context, kind, namespace, name string) error {
	gvr, ok := resourceGVR[kind]
	if !ok {
		return fmt.Errorf("unsupported resource kind: %s", kind)
	}

	dynClient, err := dynamic.NewForConfig(c.restCfg)
	if err != nil {
		return fmt.Errorf("dynamic client: %w", err)
	}

	var ns string
	if kind != "nodes" && kind != "namespaces" && kind != "pv" {
		ns = namespace
	}

	return dynClient.Resource(gvr).Namespace(ns).Delete(ctx, name, deleteOpts)
}

// cleanForRedeploy strips server-only fields from an unstructured object
// so it can be re-applied without issues.
func cleanForRedeploy(obj *unstructured.Unstructured) {
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetSelfLink("")
	obj.SetCreationTimestamp(unstructured.Unstructured{}.GetCreationTimestamp())
	obj.SetManagedFields(nil)

	// Remove status if present.
	unstructured.RemoveNestedField(obj.Object, "status")

	// Remove metadata.generation.
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
}

// resourceNameForGVK guesses the plural resource name from a GVK.
// This is a heuristic — for production we'd use a REST mapping, but for
// common built-in types this covers the vast majority of cases.
func resourceNameForGVK(gvk schema.GroupVersionKind) string {
	// Check known kinds first.
	for _, gvr := range resourceGVR {
		if gvr.Group == gvk.Group && gvr.Version == gvk.Version {
			return gvr.Resource
		}
	}
	// Heuristic: lowercase + pluralize.
	kind := strings.ToLower(gvk.Kind)
	switch {
	case strings.HasSuffix(kind, "s"):
		return kind + "es"
	case strings.HasSuffix(kind, "y"):
		return strings.TrimSuffix(kind, "y") + "ies"
	default:
		return kind + "s"
	}
}

// Predefined metav1 options.
var (
	getOpts          = metav1.GetOptions{}
	metav1CreateOpts = metav1.CreateOptions{}
	metav1UpdateOpts = metav1.UpdateOptions{}
	deleteOpts       = metav1.DeleteOptions{}
)
