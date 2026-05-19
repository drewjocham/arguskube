package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// listDeploymentsWithRetry calls Deployments(ns).List with up to 2
// retries on transient errors. The user-reported symptom — "same
// namespace works sometimes and fails sometimes" — is the classic
// signature of a single-shot K8s API call hitting a network blip,
// API-server throttle (429), or a brief 5xx during a leader-election.
// Permanent failures (Forbidden, NotFound, context canceled) short-
// circuit so we don't waste time retrying things that can't recover.
func (c *Client) listDeploymentsWithRetry(ctx context.Context, namespace string) (*appsv1.DeploymentList, error) {
	const maxAttempts = 3
	backoff := 200 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		deps, err := c.cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err == nil {
			return deps, nil
		}
		lastErr = err
		if !isTransientK8sError(err) || attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, lastErr
}

// isTransientK8sError returns true for K8s API errors that have a
// reasonable chance of succeeding on a retry. We deliberately keep
// the set narrow — retrying a 403 forever just punishes the user
// with the same eventual error after a longer wait.
func isTransientK8sError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) {
		return false
	}
	return apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err)
}

// friendlyWasteError translates a low-level K8s API error into a
// short message the user can act on. The raw error
// ("Get \"https://10.0.0.1:6443/...\": dial tcp: i/o timeout") was
// previously surfaced verbatim in the heatmap banner.
func friendlyWasteError(namespace string, err error) error {
	switch {
	case apierrors.IsForbidden(err):
		return fmt.Errorf("you don't have permission to list deployments in %q", namespace)
	case apierrors.IsUnauthorized(err):
		return fmt.Errorf("not authenticated against the cluster — sign in and retry")
	case apierrors.IsNotFound(err):
		return fmt.Errorf("namespace %q not found", namespace)
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err):
		return fmt.Errorf("cluster API timed out listing deployments in %q — retry in a moment", namespace)
	case apierrors.IsTooManyRequests(err):
		return fmt.Errorf("cluster API rate-limited the request — retry in a moment")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("listing deployments in %q took too long; cluster may be overloaded", namespace)
	default:
		return fmt.Errorf("could not list deployments in %q: %w", namespace, err)
	}
}

type WasteProfile struct {
	Namespace     string      `json:"namespace"`
	Deployments   []WasteItem `json:"deployments"`
	TotalWasteCPU string      `json:"totalWasteCPU"`
	TotalWasteMem string      `json:"totalWasteMem"`
	Score         string      `json:"score"`
}

type WasteItem struct {
	Name          string `json:"name"`
	CPURequest    string `json:"cpuRequest"`
	MemoryRequest string `json:"memoryRequest"`
	CPUUsage      string `json:"cpuUsage,omitempty"`
	MemoryUsage   string `json:"memoryUsage,omitempty"`
	WasteCPU      string `json:"wasteCPU,omitempty"`
	WasteMem      string `json:"wasteMem,omitempty"`
	Ratio         string `json:"ratio,omitempty"`
}

func (c *Client) ProfileWaste(ctx context.Context, namespace string) (*WasteProfile, error) {
	profile := &WasteProfile{Namespace: namespace}

	deps, err := c.listDeploymentsWithRetry(ctx, namespace)
	if err != nil {
		return nil, friendlyWasteError(namespace, err)
	}

	var totalWasteCPU, totalWasteMem int64

	for i := range deps.Items {
		dep := &deps.Items[i]
		for _, container := range dep.Spec.Template.Spec.Containers {
			cpuReq := container.Resources.Requests.Cpu()
			memReq := container.Resources.Requests.Memory()
			if cpuReq == nil || cpuReq.IsZero() {
				continue
			}

			cpuMilli := cpuReq.MilliValue()
			memBytes := memReq.Value()

			wasteCPU := int64(0)
			wasteMem := int64(0)

			estimatedCPUMilli := cpuMilli / 10
			estimatedMemBytes := memBytes / 10

			if cpuMilli > 100 && estimatedCPUMilli < cpuMilli/5 {
				wasteCPU = cpuMilli - estimatedCPUMilli
			}
			if memBytes > 128*1024*1024 && estimatedMemBytes < memBytes/5 {
				wasteMem = memBytes - estimatedMemBytes
			}

			totalWasteCPU += wasteCPU
			totalWasteMem += wasteMem

			ratio := float64(cpuMilli) / 100.0
			if estimatedCPUMilli > 0 {
				ratio = float64(cpuMilli) / float64(estimatedCPUMilli)
			}

			item := WasteItem{
				Name:          dep.Name + "/" + container.Name,
				CPURequest:    cpuReq.String(),
				MemoryRequest: memReq.String(),
				WasteCPU:      formatResourceMilli(wasteCPU),
				WasteMem:      formatResourceBytes(wasteMem),
				Ratio:         fmt.Sprintf("%.1fx", ratio),
			}

			if wasteCPU > 500 || wasteMem > 512*1024*1024 {
				item.CPUUsage = "estimated: " + formatResourceMilli(estimatedCPUMilli)
				item.MemoryUsage = "estimated: " + formatResourceBytes(estimatedMemBytes)
			}

			profile.Deployments = append(profile.Deployments, item)
		}
	}

	profile.TotalWasteCPU = formatResourceMilli(totalWasteCPU)
	profile.TotalWasteMem = formatResourceBytes(totalWasteMem)

	switch {
	case totalWasteCPU > 2000 || totalWasteMem > 4*1024*1024*1024:
		profile.Score = "critical"
	case totalWasteCPU > 500 || totalWasteMem > 1*1024*1024*1024:
		profile.Score = "high"
	case totalWasteCPU > 100 || totalWasteMem > 256*1024*1024:
		profile.Score = "medium"
	default:
		profile.Score = "low"
	}

	sort.Slice(profile.Deployments, func(i, j int) bool {
		return profile.Deployments[i].WasteCPU > profile.Deployments[j].WasteCPU
	})

	return profile, nil
}

func formatResourceMilli(milli int64) string {
	if milli >= 1000 {
		return fmt.Sprintf("%.1f CPU", float64(milli)/1000)
	}
	return fmt.Sprintf("%dm", milli)
}

func formatResourceBytes(b int64) string {
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1024*1024*1024))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%d MiB", b/(1024*1024))
	}
	return fmt.Sprintf("%d KiB", b/1024)
}
