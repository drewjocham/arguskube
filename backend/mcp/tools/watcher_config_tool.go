package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/djocham/kube-watcher/pkg/kube"
)

type WatcherConfigTool struct {
	BaseTool
}

func NewWatcherConfigTool(k8sManager kube.ClientInterface) *WatcherConfigTool {
	return &WatcherConfigTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *WatcherConfigTool) Name() string {
	return "update_watcher_config"
}

func (t *WatcherConfigTool) Description() string {
	return "Update the watcher engine configuration by modifying its ConfigMap. The config parameter should be a valid YAML string conforming to the watcher config schema."
}

func (t *WatcherConfigTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "config", Type: "string", Description: "YAML configuration for the watcher engine", Required: true},
		{Name: "namespace", Type: "string", Description: "Namespace where the watcher ConfigMap resides", Default: "kube-watcher"},
		{Name: "configmap", Type: "string", Description: "Name of the ConfigMap to update", Default: "watcher-config"},
	}
}

func (t *WatcherConfigTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	configYAML := t.GetStringArg(args, "config", "")
	if configYAML == "" {
		return nil, fmt.Errorf("config parameter is required and cannot be empty")
	}

	namespace := t.GetStringArg(args, "namespace", "kube-watcher")
	configmapName := t.GetStringArg(args, "configmap", "watcher-config")

	// Validate YAML by parsing into a generic map
	var dummy map[string]interface{}
	if err := yaml.Unmarshal([]byte(configYAML), &dummy); err != nil {
		return nil, fmt.Errorf("invalid YAML format: %w", err)
	}

	rawClient := t.K8sManager.GetRawInterface()
	if rawClient == nil {
		return nil, fmt.Errorf("kubernetes client not available")
	}

	// Fetch existing ConfigMap
	cm, err := rawClient.CoreV1().ConfigMaps(namespace).Get(ctx, configmapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, configmapName, err)
	}

	// Update the config.yaml data
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["config.yaml"] = configYAML

	// Apply the update
	_, err = rawClient.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update ConfigMap %s/%s: %w", namespace, configmapName, err)
	}

	return map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("ConfigMap %s/%s updated successfully", namespace, configmapName),
		"namespace": namespace,
		"configmap": configmapName,
	}, nil
}

type WatcherConfigGetTool struct {
	BaseTool
}

func NewWatcherConfigGetTool(k8sManager kube.ClientInterface) *WatcherConfigGetTool {
	return &WatcherConfigGetTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *WatcherConfigGetTool) Name() string {
	return "get_watcher_config"
}

func (t *WatcherConfigGetTool) Description() string {
	return "Retrieve the current watcher engine configuration from its ConfigMap."
}

func (t *WatcherConfigGetTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "namespace", Type: "string", Description: "Namespace where the watcher ConfigMap resides", Default: "kube-watcher"},
		{Name: "configmap", Type: "string", Description: "Name of the ConfigMap to read", Default: "watcher-config"},
	}
}

func (t *WatcherConfigGetTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	namespace := t.GetStringArg(args, "namespace", "kube-watcher")
	configmapName := t.GetStringArg(args, "configmap", "watcher-config")

	rawClient := t.K8sManager.GetRawInterface()
	if rawClient == nil {
		return nil, fmt.Errorf("kubernetes client not available")
	}

	cm, err := rawClient.CoreV1().ConfigMaps(namespace).Get(ctx, configmapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, configmapName, err)
	}

	configYAML := cm.Data["config.yaml"]
	if configYAML == "" {
		return nil, fmt.Errorf("config.yaml key not found in ConfigMap %s/%s", namespace, configmapName)
	}

	return map[string]interface{}{
		"success":   true,
		"config":    configYAML,
		"namespace": namespace,
		"configmap": configmapName,
	}, nil
}
