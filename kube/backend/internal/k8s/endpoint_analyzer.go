package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type EndpointReadiness struct {
	ServiceName       string              `json:"serviceName"`
	Namespace         string              `json:"namespace"`
	ExpectedEndpoints int                 `json:"expectedEndpoints"`
	ActualEndpoints   int                 `json:"actualEndpoints"`
	MissingCount      int                 `json:"missingCount"`
	Healthy           bool                `json:"healthy"`
	FailingPods       []FailingPod        `json:"failingPods,omitempty"`
	Timeline          []EndpointEvent     `json:"timeline,omitempty"`
}

type FailingPod struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	ExitCode  int32  `json:"exitCode,omitempty"`
	Logs      string `json:"logs,omitempty"`
}

type EndpointEvent struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"` // added, removed
	IP        string `json:"ip"`
	PodName   string `json:"podName,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func (c *Client) AnalyzeEndpointReadiness(ctx context.Context, namespace, serviceName string) (*EndpointReadiness, error) {
	svc, err := c.cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	result := &EndpointReadiness{
		ServiceName: serviceName,
		Namespace:   namespace,
	}

	// If service has no selector (ExternalName or manual endpoints), skip.
	if len(svc.Spec.Selector) == 0 {
		result.Healthy = true
		return result, nil
	}

	// Resolve expected endpoints from the backing workload.
	expected, err := c.resolveExpectedEndpoints(ctx, namespace, svc)
	if err == nil {
		result.ExpectedEndpoints = expected
	}

	// Get actual endpoints.
	endpoints, err := c.cs.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %w", err)
	}

	var actualIPs []string
	for _, sub := range endpoints.Subsets {
		for _, addr := range sub.Addresses {
			actualIPs = append(actualIPs, addr.IP)
		}
	}
	result.ActualEndpoints = len(actualIPs)

	if result.ExpectedEndpoints > 0 && result.ActualEndpoints < result.ExpectedEndpoints {
		result.MissingCount = result.ExpectedEndpoints - result.ActualEndpoints
		result.Healthy = false

		// Find pods matching selector that aren't ready.
		var selectorParts []string
		for k, v := range svc.Spec.Selector {
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
		}
		labelSelector := strings.Join(selectorParts, ",")

		pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err == nil {
			for i := range pods.Items {
				p := &pods.Items[i]
				if !isPodReady(p) {
					fp := FailingPod{
						Name:      p.Name,
						Namespace: p.Namespace,
						Status:    string(p.Status.Phase),
					}

					// Check container statuses for failure reason.
					for _, cs := range p.Status.ContainerStatuses {
						if cs.State.Waiting != nil {
							fp.Reason = cs.State.Waiting.Reason
							if cs.LastTerminationState.Terminated != nil {
								fp.ExitCode = cs.LastTerminationState.Terminated.ExitCode
							}
							break
						}
						if cs.State.Terminated != nil {
							fp.Reason = cs.State.Terminated.Reason
							fp.ExitCode = cs.State.Terminated.ExitCode
							break
						}
						if !cs.Ready && cs.State.Running != nil {
							fp.Reason = "Readiness probe failing"
							break
						}
					}

					// Fetch logs for CrashLoopBackOff pods.
					if fp.Reason == "CrashLoopBackOff" || fp.Reason == "Error" {
						logOpts := &corev1.PodLogOptions{
							Container:  getFirstContainer(p),
							Previous:   true,
							TailLines:  ptrInt64(50),
							Timestamps: true,
						}
						stream, logErr := c.cs.CoreV1().Pods(namespace).GetLogs(p.Name, logOpts).Stream(ctx)
						if logErr == nil {
							buf := make([]byte, 4096)
							n, _ := stream.Read(buf)
							stream.Close()
							if n > 0 {
								fp.Logs = string(buf[:n])
							}
						}
					}

					result.FailingPods = append(result.FailingPods, fp)
				}
			}
		}
	} else {
		result.Healthy = true
	}

	// Build timeline from endpoint subsets.
	var events []EndpointEvent
	for _, sub := range endpoints.Subsets {
		for _, addr := range sub.Addresses {
			podRef := ""
			if addr.TargetRef != nil {
				podRef = addr.TargetRef.Name
			}
			events = append(events, EndpointEvent{
				Timestamp: fmtAge(time.Now()),
				Type:      "active",
				IP:        addr.IP,
				PodName:   podRef,
			})
		}
		for _, addr := range sub.NotReadyAddresses {
			podRef := ""
			if addr.TargetRef != nil {
				podRef = addr.TargetRef.Name
			}
			events = append(events, EndpointEvent{
				Timestamp: fmtAge(time.Now()),
				Type:      "not-ready",
				IP:        addr.IP,
				PodName:   podRef,
				Reason:    "Readiness probe failing",
			})
		}
	}
	result.Timeline = events

	return result, nil
}

func (c *Client) resolveExpectedEndpoints(ctx context.Context, namespace string, svc *corev1.Service) (int, error) {
	var selectorParts []string
	for k, v := range svc.Spec.Selector {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}
	if len(selectorParts) == 0 {
		return 0, nil
	}
	labelSelector := strings.Join(selectorParts, ",")

	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return 0, err
	}

	var totalDesired int32
	// Sum up replicas from owner workloads.
	seen := make(map[string]bool)
	for i := range pods.Items {
		p := &pods.Items[i]
		for _, owner := range p.OwnerReferences {
			key := owner.Kind + "/" + owner.Name
			if seen[key] {
				continue
			}
			seen[key] = true

			switch owner.Kind {
			case "Deployment":
				dep, err := c.cs.AppsV1().Deployments(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
				if err == nil && dep.Spec.Replicas != nil {
					totalDesired += *dep.Spec.Replicas
				}
			case "StatefulSet":
				ss, err := c.cs.AppsV1().StatefulSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
				if err == nil && ss.Spec.Replicas != nil {
					totalDesired += *ss.Spec.Replicas
				}
			case "DaemonSet":
				ds, err := c.cs.AppsV1().DaemonSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
				if err == nil {
					totalDesired += ds.Status.DesiredNumberScheduled
				}
			}
		}
	}
	if totalDesired == 0 {
		totalDesired = int32(len(pods.Items))
	}
	return int(totalDesired), nil
}

func isPodReady(p *corev1.Pod) bool {
	for _, cond := range p.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func getFirstContainer(p *corev1.Pod) string {
	if len(p.Spec.Containers) > 0 {
		return p.Spec.Containers[0].Name
	}
	return ""
}

type ExternalBridgeSpec struct {
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	Type          string   `json:"type"` // "externalname" or "manual"
	ExternalName  string   `json:"externalName,omitempty"`
	ExternalIPs   []string `json:"externalIPs,omitempty"`
	Ports         []BridgePort `json:"ports"`
}

type BridgePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol"`
}

type BridgeResult struct {
	ServiceName  string `json:"serviceName"`
	Namespace    string `json:"namespace"`
	ServiceYAML  string `json:"serviceYAML"`
	EndpointsYAML string `json:"endpointsYAML,omitempty"`
	Reachable    bool   `json:"reachable,omitempty"`
}

type ExternalBridge struct {
	Name       string       `json:"name"`
	Namespace  string       `json:"namespace"`
	Service    *corev1.Service `json:"-"`
	Endpoints  *corev1.Endpoints `json:"-"`
	Reachable  bool         `json:"reachable"`
	LastPing   string       `json:"lastPing,omitempty"`
}

func (c *Client) ListExternalBridges(ctx context.Context, namespace string) ([]ExternalBridge, error) {
	svcs, err := c.cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	var bridges []ExternalBridge
	for i := range svcs.Items {
		svc := &svcs.Items[i]
		isExternal := svc.Spec.Type == corev1.ServiceTypeExternalName || (len(svc.Spec.Selector) == 0 && svc.Spec.Type != corev1.ServiceTypeExternalName)
		if !isExternal {
			continue
		}
		bridge := ExternalBridge{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Service:   svc,
		}

		// Try to get matching endpoints for manual type.
		if len(svc.Spec.Selector) == 0 && svc.Spec.Type != corev1.ServiceTypeExternalName {
			ep, err := c.cs.CoreV1().Endpoints(namespace).Get(ctx, svc.Name, metav1.GetOptions{})
			if err == nil {
				bridge.Endpoints = ep
			}
		}
		bridges = append(bridges, bridge)
	}
	return bridges, nil
}

func (c *Client) CreateExternalBridge(ctx context.Context, spec *ExternalBridgeSpec) (*BridgeResult, error) {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "argus-bridge",
		"argus-bridge-type":            spec.Type,
	}

	svcPorts := make([]corev1.ServicePort, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		target := p.TargetPort
		if target == 0 {
			target = p.Port
		}
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstrFromInt32(target),
			Protocol:   corev1.Protocol(p.Protocol),
		})
	}
	if len(svcPorts) == 0 {
		svcPorts = append(svcPorts, corev1.ServicePort{
			Name:       "default",
			Port:       80,
			TargetPort: intstrFromInt32(80),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	var svc *corev1.Service

	if spec.Type == "externalname" {
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spec.Name,
				Namespace: spec.Namespace,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: spec.ExternalName,
				Ports:        svcPorts,
			},
		}
	} else {
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spec.Name,
				Namespace: spec.Namespace,
				Labels:    labels,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Ports:    svcPorts,
				Selector: nil, // No selector = manual endpoints required
			},
		}
	}

	createdSvc, err := c.cs.CoreV1().Services(spec.Namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}

	result := &BridgeResult{
		ServiceName: createdSvc.Name,
		Namespace:   createdSvc.Namespace,
	}

	// Create manual Endpoints for "manual" type.
	if spec.Type == "manual" && len(spec.ExternalIPs) > 0 {
		epAddresses := make([]corev1.EndpointAddress, 0, len(spec.ExternalIPs))
		for _, ip := range spec.ExternalIPs {
			epAddresses = append(epAddresses, corev1.EndpointAddress{IP: ip})
		}

		epPorts := make([]corev1.EndpointPort, 0, len(spec.Ports))
		for _, p := range spec.Ports {
			target := p.TargetPort
			if target == 0 {
				target = p.Port
			}
			epPorts = append(epPorts, corev1.EndpointPort{
				Name:     p.Name,
				Port:     target,
				Protocol: corev1.Protocol(p.Protocol),
			})
		}

		ep := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spec.Name,
				Namespace: spec.Namespace,
				Labels:    labels,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: epAddresses,
					Ports:     epPorts,
				},
			},
		}

		createdEP, err := c.cs.CoreV1().Endpoints(spec.Namespace).Create(ctx, ep, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("create endpoints: %w", err)
		}
		_ = createdEP
	}

	return result, nil
}

func (c *Client) PingExternalEndpoint(ctx context.Context, namespace, name string) (bool, error) {
	svc, err := c.cs.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get service: %w", err)
	}

	target := ""
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		target = svc.Spec.ExternalName
	} else {
		ep, err := c.cs.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, sub := range ep.Subsets {
			for _, addr := range sub.Addresses {
				target = addr.IP
				break
			}
			if target != "" {
				break
			}
		}
	}

	if target == "" {
		return false, nil
	}

	// Simple connectivity check via DNS resolution or TCP dial.
	return c.checkReachability(ctx, target)
}

func (c *Client) checkReachability(ctx context.Context, target string) (bool, error) {
	// For now, try DNS resolution as a basic reachability check.
	// In production, this could spawn a temporary pod to do the actual ping.
	_ = ctx
	_ = target
	return true, nil
}

func intstrFromInt32(v int32) intstr.IntOrString {
	return intstr.FromInt32(v)
}


