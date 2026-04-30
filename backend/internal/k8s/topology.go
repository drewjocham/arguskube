package k8s

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyNode represents a vertex in the topology graph.
type TopologyNode struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // node, deployment, pod, service
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"` // ok, warn, crit
}

// TopologyEdge connects two topology nodes.
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

// TopologyResult is the full topology graph.
type TopologyResult struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// BuildTopology constructs a topology graph from live cluster state.
// Maps: Nodes → Deployments → Pods, Services → Deployments.
func (c *Client) BuildTopology(ctx context.Context, namespace string) (*TopologyResult, error) {
	ns := namespace
	if ns == "" {
		ns = c.cfg.Kubernetes.Namespace
	}

	var result TopologyResult

	// Fetch nodes.
	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	for _, n := range nodes.Items {
		status := "ok"
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
				status = "crit"
			}
			if (cond.Type == corev1.NodeDiskPressure || cond.Type == corev1.NodeMemoryPressure) && cond.Status == corev1.ConditionTrue {
				if status != "crit" {
					status = "warn"
				}
			}
		}
		result.Nodes = append(result.Nodes, TopologyNode{
			ID:     "node/" + n.Name,
			Kind:   "node",
			Name:   n.Name,
			Status: status,
		})
	}

	// Fetch deployments.
	deploys, err := c.cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, d := range deploys.Items {
		status := "ok"
		if d.Status.ReadyReplicas < d.Status.Replicas {
			status = "warn"
		}
		if d.Status.ReadyReplicas == 0 && d.Status.Replicas > 0 {
			status = "crit"
		}
		result.Nodes = append(result.Nodes, TopologyNode{
			ID:        fmt.Sprintf("deploy/%s/%s", d.Namespace, d.Name),
			Kind:      "deployment",
			Name:      d.Name,
			Namespace: d.Namespace,
			Status:    status,
		})
	}

	// Fetch pods and link to nodes + deployments.
	pods, err := c.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	for _, p := range pods.Items {
		status := "ok"
		switch p.Status.Phase {
		case corev1.PodFailed:
			status = "crit"
		case corev1.PodPending:
			status = "warn"
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				status = "crit"
			}
		}

		podID := fmt.Sprintf("pod/%s/%s", p.Namespace, p.Name)
		result.Nodes = append(result.Nodes, TopologyNode{
			ID:        podID,
			Kind:      "pod",
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    status,
		})

		// Edge: pod → node.
		if p.Spec.NodeName != "" {
			result.Edges = append(result.Edges, TopologyEdge{
				Source: podID,
				Target: "node/" + p.Spec.NodeName,
				Label:  "runs-on",
			})
		}

		// Edge: deployment → pod (via owner references).
		for _, ref := range p.OwnerReferences {
			if ref.Kind == "ReplicaSet" {
				// Find the owning deployment for this replicaset.
				rsName := ref.Name
				deployName := rsNameToDeploy(rsName)
				deployID := fmt.Sprintf("deploy/%s/%s", p.Namespace, deployName)
				result.Edges = append(result.Edges, TopologyEdge{
					Source: deployID,
					Target: podID,
					Label:  "manages",
				})
			}
		}
	}

	// Fetch services and link to deployments via label selectors.
	svcs, err := c.cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.logger.Debug("failed to list services for topology", "error", err)
	} else {
		for _, svc := range svcs.Items {
			if svc.Spec.ClusterIP == "None" && svc.Name == "kubernetes" {
				continue
			}
			svcID := fmt.Sprintf("svc/%s/%s", svc.Namespace, svc.Name)
			result.Nodes = append(result.Nodes, TopologyNode{
				ID:        svcID,
				Kind:      "service",
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Status:    "ok",
			})

			// Match service selector to deployments.
			if svc.Spec.Selector != nil {
				for _, d := range deploys.Items {
					if matchesLabels(svc.Spec.Selector, d.Spec.Template.Labels) {
						deployID := fmt.Sprintf("deploy/%s/%s", d.Namespace, d.Name)
						result.Edges = append(result.Edges, TopologyEdge{
							Source: svcID,
							Target: deployID,
							Label:  "routes-to",
						})
					}
				}
			}
		}
	}

	return &result, nil
}

// rsNameToDeploy derives the deployment name from a replicaset name.
// ReplicaSets are named "<deployment>-<hash>", so strip the last segment.
func rsNameToDeploy(rsName string) string {
	for i := len(rsName) - 1; i >= 0; i-- {
		if rsName[i] == '-' {
			return rsName[:i]
		}
	}
	return rsName
}

// matchesLabels returns true if all selector keys match the labels.
func matchesLabels(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return len(selector) > 0
}

// --- Application abstraction (ArgoCD-like) ---

// Application represents a deployment treated as an "application" with rollout status.
type Application struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	SyncStatus    string `json:"syncStatus"`    // Synced, OutOfSync
	HealthStatus  string `json:"healthStatus"`  // Healthy, Degraded, Progressing
	Replicas      int32  `json:"replicas"`
	ReadyReplicas int32  `json:"readyReplicas"`
	Image         string `json:"image"`
	LastSync      string `json:"lastSync"`
}

// ListApplications returns all deployments as "applications".
func (c *Client) ListApplications(ctx context.Context, namespace string) ([]Application, error) {
	ns := namespace
	if ns == "" {
		ns = c.cfg.Kubernetes.Namespace
	}

	deploys, err := c.cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	var apps []Application
	for _, d := range deploys.Items {
		syncStatus := "Synced"
		if d.Status.UpdatedReplicas < d.Status.Replicas {
			syncStatus = "OutOfSync"
		}

		healthStatus := "Healthy"
		if d.Status.ReadyReplicas == 0 && d.Status.Replicas > 0 {
			healthStatus = "Degraded"
		} else if d.Status.ReadyReplicas < d.Status.Replicas {
			healthStatus = "Progressing"
		}

		image := ""
		if len(d.Spec.Template.Spec.Containers) > 0 {
			image = d.Spec.Template.Spec.Containers[0].Image
		}

		lastSync := formatAge(d.Status.Conditions)

		apps = append(apps, Application{
			Name:          d.Name,
			Namespace:     d.Namespace,
			SyncStatus:    syncStatus,
			HealthStatus:  healthStatus,
			Replicas:      d.Status.Replicas,
			ReadyReplicas: d.Status.ReadyReplicas,
			Image:         image,
			LastSync:      lastSync,
		})
	}

	return apps, nil
}

// RestartDeployment triggers a rollout restart by patching the pod template annotation.
func (c *Client) RestartDeployment(ctx context.Context, namespace, name string) error {
	deploy, err := c.cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = c.cs.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("restart deployment: %w", err)
	}
	return nil
}

// formatAge returns a human-readable age from deployment conditions.
func formatAge(conditions []appsv1.DeploymentCondition) string {
	for _, c := range conditions {
		if c.Type == appsv1.DeploymentProgressing {
			age := time.Since(c.LastTransitionTime.Time)
			switch {
			case age < time.Minute:
				return "just now"
			case age < time.Hour:
				return fmt.Sprintf("%dm ago", int(age.Minutes()))
			case age < 24*time.Hour:
				return fmt.Sprintf("%dh ago", int(age.Hours()))
			default:
				return fmt.Sprintf("%dd ago", int(age.Hours()/24))
			}
		}
	}
	return "—"
}
