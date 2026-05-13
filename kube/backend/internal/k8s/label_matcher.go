package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LabelDiffResult struct {
	ServiceName  string          `json:"serviceName"`
	Namespace    string          `json:"namespace"`
	Selector     map[string]string `json:"selector"`
	Matches      []LabelMatch    `json:"matches"`
	Mismatches   []LabelMismatch `json:"mismatches"`
	OrphanedPods []OrphanedPod   `json:"orphanedPods,omitempty"`
	HasIssues    bool            `json:"hasIssues"`
}

type LabelMatch struct {
	PodName  string `json:"podName"`
	Complete bool   `json:"complete"`
}

type LabelMismatch struct {
	PodName       string `json:"podName"`
	SelectorKey   string `json:"selectorKey"`
	SelectorValue string `json:"selectorValue"`
	ActualValue   string `json:"actualValue"`
	Missing       bool   `json:"missing"`
}

type OrphanedPod struct {
	PodName       string `json:"podName"`
	Namespace     string `json:"namespace"`
	HasMatchingSvc bool   `json:"hasMatchingSvc"`
}

func (c *Client) AnalyzeLabelMatch(ctx context.Context, namespace, serviceName string) (*LabelDiffResult, error) {
	svc, err := c.cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	if len(svc.Spec.Selector) == 0 {
		return &LabelDiffResult{
			ServiceName: serviceName,
			Namespace:   namespace,
			Selector:    svc.Spec.Selector,
			HasIssues:   false,
		}, nil
	}

	result := &LabelDiffResult{
		ServiceName: serviceName,
		Namespace:   namespace,
		Selector:    svc.Spec.Selector,
	}

	// Fetch all pods in the namespace.
	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	matchedPodNames := make(map[string]bool)

	for i := range pods.Items {
		p := &pods.Items[i]
		podLabels := p.Labels
		allMatch := true
		hasAnyMatch := false

		for sk, sv := range svc.Spec.Selector {
			pv, exists := podLabels[sk]
			if !exists {
				result.Mismatches = append(result.Mismatches, LabelMismatch{
					PodName:       p.Name,
					SelectorKey:   sk,
					SelectorValue: sv,
					ActualValue:   "",
					Missing:       true,
				})
				allMatch = false
			} else if pv != sv {
				result.Mismatches = append(result.Mismatches, LabelMismatch{
					PodName:       p.Name,
					SelectorKey:   sk,
					SelectorValue: sv,
					ActualValue:   pv,
					Missing:       false,
				})
				allMatch = false
			} else {
				hasAnyMatch = true
			}
		}

		if hasAnyMatch {
			matchedPodNames[p.Name] = allMatch
			result.Matches = append(result.Matches, LabelMatch{
				PodName:  p.Name,
				Complete: allMatch,
			})
		}
	}

	result.HasIssues = len(result.Mismatches) > 0

	// Check for orphaned endpoints.
	ep, err := c.cs.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err == nil {
		activePodIPs := make(map[string]bool)
		for i := range pods.Items {
			activePodIPs[pods.Items[i].Status.PodIP] = true
		}

		for _, sub := range ep.Subsets {
			for _, addr := range sub.Addresses {
				if !activePodIPs[addr.IP] {
					podName := ""
					if addr.TargetRef != nil {
						podName = addr.TargetRef.Name
					}
					result.OrphanedPods = append(result.OrphanedPods, OrphanedPod{
						PodName:       podName,
						Namespace:     namespace,
						HasMatchingSvc: false,
					})
				}
			}
		}
	}

	return result, nil
}

func (c *Client) FindOrphanedEndpoints(ctx context.Context, namespace string) ([]OrphanedEndpoint, error) {
	endpoints, err := c.cs.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}

	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	activePodIPs := make(map[string]bool)
	for i := range pods.Items {
		activePodIPs[pods.Items[i].Status.PodIP] = true
	}

	var orphans []OrphanedEndpoint
	for i := range endpoints.Items {
		ep := &endpoints.Items[i]
		for _, sub := range ep.Subsets {
			for _, addr := range sub.Addresses {
				if !activePodIPs[addr.IP] {
					podName := ""
					if addr.TargetRef != nil {
						podName = addr.TargetRef.Name
					}
					orphans = append(orphans, OrphanedEndpoint{
						EndpointName: ep.Name,
						IP:           addr.IP,
						PodName:      podName,
						Namespace:    ep.Namespace,
					})
				}
			}
		}
	}

	return orphans, nil
}

type OrphanedEndpoint struct {
	EndpointName string `json:"endpointName"`
	IP           string `json:"ip"`
	PodName      string `json:"podName,omitempty"`
	Namespace    string `json:"namespace"`
}

func (c *Client) ListServiceSelectors(ctx context.Context, namespace string) ([]ServiceSelectorInfo, error) {
	svcs, err := c.cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	var infos []ServiceSelectorInfo
	for i := range svcs.Items {
		svc := &svcs.Items[i]
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		info := ServiceSelectorInfo{
			ServiceName: svc.Name,
			Namespace:   svc.Namespace,
			Selector:    make(map[string]string),
		}
		for k, v := range svc.Spec.Selector {
			info.Selector[k] = v
		}

		// Count matching pods.
		var parts []string
		for k, v := range svc.Spec.Selector {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: strings.Join(parts, ","),
		})
		if err == nil {
			info.MatchingPods = len(pods.Items)
		}

		infos = append(infos, info)
	}

	return infos, nil
}

type ServiceSelectorInfo struct {
	ServiceName  string            `json:"serviceName"`
	Namespace    string            `json:"namespace"`
	Selector     map[string]string `json:"selector"`
	MatchingPods int               `json:"matchingPods"`
}
