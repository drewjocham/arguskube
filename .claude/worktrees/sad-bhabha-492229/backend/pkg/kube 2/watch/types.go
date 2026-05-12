// Package watch provides Kubernetes resource watchers and alert generation
// for the MCP server subsystem.
package watch

import (
	"fmt"
	"time"
)

// AlertKind classifies the source of an alert.
type AlertKind string

const (
	AlertKindPod   AlertKind = "pod"
	AlertKindNode  AlertKind = "node"
	AlertKindEvent AlertKind = "event"
)

// Alert represents a detected issue in the cluster.
type Alert struct {
	Kind       AlertKind `json:"kind"`
	Name       string    `json:"name"`
	Namespace  string    `json:"namespace"`
	Severity   string    `json:"severity"`
	Reason     string    `json:"reason"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Key returns a stable deduplication key for this alert.
func (a Alert) Key() string {
	return fmt.Sprintf("%s/%s/%s/%s", a.Kind, a.Namespace, a.Name, a.Reason)
}
