package tools

import (
	"context"
	"errors"

	"github.com/argues/kube-watcher/pkg/kube"
)

var (
	ErrNamespaceRequired = errors.New("namespace is required")
	ErrPodNameRequired   = errors.New("pod_name is required")
)

type PodLogsTool struct {
	BaseTool
}

func NewPodLogsTool(k8sManager kube.ClientInterface) *PodLogsTool {
	return &PodLogsTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *PodLogsTool) Name() string {
	return "get_pod_logs"
}

func (t *PodLogsTool) Description() string {
	return "Fetch logs for a specific pod/container in a namespace."
}

func (t *PodLogsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "namespace", Type: "string", Description: "Namespace containing the pod.", Required: true},
		{Name: "pod_name", Type: "string", Description: "Pod name to fetch logs for.", Required: true},
		{Name: "container", Type: "string", Description: "Optional container name for multi-container pods."},
		{Name: "tail_lines", Type: "number", Description: "Number of trailing log lines to return.", Default: 200},
		{Name: "since_seconds", Type: "number", Description: "Only return logs newer than this many seconds.", Default: 0},
		{Name: "previous", Type: "boolean", Description: "Return logs from the previous container instance.", Default: false},
	}
}

func (t *PodLogsTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	namespace := t.GetStringArg(args, "namespace", "")
	if namespace == "" {
		return nil, ErrNamespaceRequired
	}

	podName := t.GetStringArg(args, "pod_name", "")
	if podName == "" {
		return nil, ErrPodNameRequired
	}

	container := t.GetStringArg(args, "container", "")
	tailLines := int64(t.GetIntArg(args, "tail_lines", 200))
	sinceSeconds := int64(t.GetIntArg(args, "since_seconds", 0))
	previous := t.GetBoolArg(args, "previous", false)

	logs, err := t.K8sManager.GetPodLogs(ctx, namespace, podName, container, tailLines, sinceSeconds, previous)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"namespace":     namespace,
		"pod_name":      podName,
		"container":     container,
		"tail_lines":    tailLines,
		"since_seconds": sinceSeconds,
		"previous":      previous,
		"logs":          logs,
	}, nil
}
