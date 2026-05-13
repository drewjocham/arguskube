package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CrashInfo struct {
	ExitCode  int32  `json:"exitCode"`
	Reason    string `json:"reason"`
	Logs      string `json:"logs"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

type DuplicateResult struct {
	Deployment string `json:"deployment"`
	Namespace  string `json:"namespace"`
}

type NodeCapacity struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

func (c *Client) DuplicateDeployment(ctx context.Context, namespace, name, targetNamespace string) (*DuplicateResult, error) {
	dep, err := c.cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get deployment %s/%s: %w", namespace, name, err)
	}

	clone := dep.DeepCopy()
	clone.ResourceVersion = ""
	clone.UID = ""
	clone.SelfLink = ""
	clone.CreationTimestamp = metav1.Time{}
	clone.Generation = 0
	clone.ManagedFields = nil
	clone.Namespace = targetNamespace

	for k := range clone.Annotations {
		if strings.HasPrefix(k, "deployment.kubernetes.io/") || strings.HasPrefix(k, "kubectl.kubernetes.io/") {
			delete(clone.Annotations, k)
		}
	}

	created, err := c.cs.AppsV1().Deployments(targetNamespace).Create(ctx, clone, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create deployment in %s: %w", targetNamespace, err)
	}

	return &DuplicateResult{
		Deployment: created.Name,
		Namespace:  targetNamespace,
	}, nil
}

func (c *Client) CreateJobFromCronJob(ctx context.Context, namespace, cronJobName string) (*batchv1.Job, error) {
	cj, err := c.cs.BatchV1().CronJobs(namespace).Get(ctx, cronJobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get cronjob %s/%s: %w", namespace, cronJobName, err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cj.Name + "-manual-",
			Namespace:    namespace,
			Labels:       cj.Spec.JobTemplate.Labels,
			Annotations:  cj.Spec.JobTemplate.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       cj.Name,
					UID:        cj.UID,
				},
			},
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}

	created, err := c.cs.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create job from cronjob: %w", err)
	}

	return created, nil
}

func (c *Client) GetCrashLogs(ctx context.Context, namespace, podName, containerName string) (*CrashInfo, error) {
	pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod %s/%s: %w", namespace, podName, err)
	}

	container := containerName
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}

	var exitCode int32
	var reason string
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == container {
			if cs.State.Waiting != nil {
				reason = cs.State.Waiting.Reason
				if cs.LastTerminationState.Terminated != nil {
					exitCode = cs.LastTerminationState.Terminated.ExitCode
					if reason == "" {
						reason = cs.LastTerminationState.Terminated.Reason
					}
				}
			}
			if cs.State.Terminated != nil {
				exitCode = cs.State.Terminated.ExitCode
				reason = cs.State.Terminated.Reason
			}
			if cs.LastTerminationState.Terminated != nil && exitCode == 0 {
				exitCode = cs.LastTerminationState.Terminated.ExitCode
				if reason == "" {
					reason = cs.LastTerminationState.Terminated.Reason
				}
			}
			break
		}
	}

	logOpts := &corev1.PodLogOptions{
		Container:  container,
		Previous:   true,
		TailLines:  ptrInt64(100),
		Timestamps: true,
	}

	logStream, err := c.cs.CoreV1().Pods(namespace).GetLogs(podName, logOpts).Stream(ctx)
	if err != nil {
		return &CrashInfo{
			ExitCode:  exitCode,
			Reason:    reason,
			Logs:      fmt.Sprintf("(unable to fetch logs: %s)", err),
			Pod:       podName,
			Container: container,
		}, nil
	}
	defer logStream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, logStream); err != nil {
		return &CrashInfo{
			ExitCode:  exitCode,
			Reason:    reason,
			Logs:      fmt.Sprintf("(log read error: %s)", err),
			Pod:       podName,
			Container: container,
		}, nil
	}

	logs := buf.String()
	if len(logs) > 10000 {
		logs = logs[len(logs)-10000:]
	}

	return &CrashInfo{
		ExitCode:  exitCode,
		Reason:    reason,
		Logs:      logs,
		Pod:       podName,
		Container: container,
	}, nil
}

func (c *Client) SuggestResources(ctx context.Context) ([]NodeCapacity, error) {
	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes for capacity: %w", err)
	}

	if len(nodes.Items) == 0 {
		return nil, nil
	}

	capacities := make([]NodeCapacity, 0, len(nodes.Items))
	for i := range nodes.Items {
		n := &nodes.Items[i]
		cpu := n.Status.Capacity.Cpu().String()
		capacities = append(capacities, NodeCapacity{
			CPU:    cpu,
			Memory: formatNodeMemory(n.Status.Capacity.Memory().Value()),
		})
	}

	return capacities, nil
}

func formatNodeMemory(bytes int64) string {
	const gi = 1 << 30
	if bytes >= gi {
		return fmt.Sprintf("%dGi", (bytes+gi-1)/gi)
	}
	return fmt.Sprintf("%dMi", bytes/(1<<20))
}

func ptrInt64(i int64) *int64 {
	return &i
}

type ResourceSuggestion struct {
	Profile      ResourceProfile `json:"profile"`
	NodeCapacity []NodeCapacity  `json:"nodeCapacity,omitempty"`
}

func (c *Client) GetResourceSuggestion(ctx context.Context, profile string) (*ResourceSuggestion, error) {
	suggestion := &ResourceSuggestion{}

	profile = strings.ToLower(profile)
	p, ok := GetTShirtSize(profile)
	if !ok {
		defaultProfiles := TShirtSizes()
		keys := make([]string, 0, len(defaultProfiles))
		for k := range defaultProfiles {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("unknown profile %q, available: %s", profile, strings.Join(keys, ", "))
	}
	suggestion.Profile = p

	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil && len(nodes.Items) > 0 {
		capacities := make([]NodeCapacity, 0, len(nodes.Items))
		for i := range nodes.Items {
			n := &nodes.Items[i]
			capacities = append(capacities, NodeCapacity{
				CPU:    n.Status.Capacity.Cpu().String(),
				Memory: formatNodeMemory(n.Status.Capacity.Memory().Value()),
			})
		}
		suggestion.NodeCapacity = capacities
	}

	return suggestion, nil
}
