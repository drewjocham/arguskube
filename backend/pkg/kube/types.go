package kube

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// NodeInfo contains relevant details about a cluster node.
type NodeInfo struct {
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Age         time.Duration     `json:"age"`
	Conditions  []NodeCondition   `json:"conditions,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Taints      []corev1.Taint    `json:"taints,omitempty"`
	Allocatable map[string]string `json:"allocatable,omitempty"`
}

// NodeCondition is a simplified representation of a node condition.
type NodeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// PodInfo contains relevant details about a pod.
type PodInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Phase        string            `json:"phase"`
	Status       string            `json:"status"`
	NodeName     string            `json:"node_name"`
	Age          time.Duration     `json:"age"`
	RestartCount int32             `json:"restart_count"`
	Labels       map[string]string `json:"labels,omitempty"`
	Containers   []ContainerInfo   `json:"containers,omitempty"`
}

// ContainerInfo contains relevant details about a container.
type ContainerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`
}

// EventInfo contains relevant details about a Kubernetes event.
type EventInfo struct {
	Type          string    `json:"type"`
	Reason        string    `json:"reason"`
	ObjectKind    string    `json:"object_kind"`
	ObjectName    string    `json:"object_name"`
	Message       string    `json:"message"`
	LastTimestamp  time.Time `json:"last_timestamp"`
	Count         int32     `json:"count"`
}

// ServiceInfo contains relevant details about a service.
type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
}

// ResourceQuotaInfo contains relevant details about a resource quota.
type ResourceQuotaInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Hard      map[string]string `json:"hard,omitempty"`
	Used      map[string]string `json:"used,omitempty"`
}

// ClusterInfo contains high-level cluster metadata.
type ClusterInfo struct {
	Version   string `json:"version"`
	NodeCount int    `json:"node_count"`
}
