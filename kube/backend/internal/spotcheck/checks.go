package spotcheck

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/argues/argus/internal/alerts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MetricsProvider is a thin abstraction over the cached metrics on App
// so the metrics check stays testable without a live k8s connection.
type MetricsProvider interface {
	CurrentMetrics() *alerts.ClusterMetrics
}

// K8sClient is the subset of k8s.Client that NodeReadyCheck needs.
// Using an interface makes the check testable without a live cluster.
type K8sClient interface {
	GetClientset() kubernetes.Interface
}

// NodeReadyCheck flags any node that is not Ready or has memory/disk
// pressure conditions set. Cheap — just lists nodes via the API and
// reads conditions.
type NodeReadyCheck struct {
	K8s K8sClient
}

func (NodeReadyCheck) Name() string        { return "nodes-ready" }
func (NodeReadyCheck) Description() string { return "Checking node readiness…" }

func (c NodeReadyCheck) Run(ctx context.Context) (*Finding, error) {
	if c.K8s == nil {
		return nil, fmt.Errorf("no cluster connected")
	}
	nodes, err := c.K8s.GetClientset().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var notReady, pressured []string
	for _, n := range nodes.Items {
		ready := false
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				ready = true
			}
			// Memory / disk / PID pressure all warrant attention.
			if (cond.Type == corev1.NodeMemoryPressure ||
				cond.Type == corev1.NodeDiskPressure ||
				cond.Type == corev1.NodePIDPressure) &&
				cond.Status == corev1.ConditionTrue {
				pressured = append(pressured, fmt.Sprintf("%s: %s", n.Name, cond.Type))
			}
		}
		if !ready {
			notReady = append(notReady, n.Name)
		}
	}

	if len(notReady) == 0 && len(pressured) == 0 {
		return nil, nil // silent pass
	}

	body := strings.Builder{}
	if len(notReady) > 0 {
		body.WriteString("**Not Ready nodes:** ")
		body.WriteString(strings.Join(notReady, ", "))
		body.WriteString("\n\n")
	}
	if len(pressured) > 0 {
		body.WriteString("**Pressured nodes:**\n")
		for _, p := range pressured {
			body.WriteString("- " + p + "\n")
		}
	}

	sev := SevWarn
	if len(notReady) > 0 {
		sev = SevError
	}
	return &Finding{
		Severity: sev,
		Title:    fmt.Sprintf("%d node issue(s) detected", len(notReady)+len(pressured)),
		Body:     body.String(),
		Meta: map[string]any{
			"notReady":  notReady,
			"pressured": pressured,
			"total":     len(nodes.Items),
		},
	}, nil
}

// MetricsCheck looks at the cached cluster metrics and flags health
// percentage breaches. Uses the cached snapshot rather than re-fetching
// so we don't double the load on the API server every interval.
type MetricsCheck struct {
	Source MetricsProvider

	// MinHealthPct below this triggers a warning. Default 95.
	MinHealthPct float64
	// MaxErrorRate above this triggers a warning. Default 5.
	MaxErrorRate float64
}

func (MetricsCheck) Name() string        { return "cluster-metrics" }
func (MetricsCheck) Description() string { return "Reviewing cluster metrics…" }

func (c MetricsCheck) Run(ctx context.Context) (*Finding, error) {
	m := c.Source.CurrentMetrics()
	if m == nil {
		// No metrics cached yet — not a failure, just nothing to say.
		return nil, nil
	}
	minH := c.MinHealthPct
	if minH == 0 {
		minH = 95
	}
	maxE := c.MaxErrorRate
	if maxE == 0 {
		maxE = 5
	}

	if m.PodsTotal == 0 {
		return nil, nil // empty cluster
	}

	if m.PodHealthPct >= minH && m.ErrorRate <= maxE && m.WarningEvents < 10 {
		return nil, nil
	}

	body := fmt.Sprintf(
		"Pod health: **%.1f%%** (target ≥ %.0f%%)\nError rate: **%.1f%%** (target ≤ %.0f%%)\nWarning events (30m): **%d**\nRunning / Total: %d / %d",
		m.PodHealthPct, minH, m.ErrorRate, maxE, m.WarningEvents, m.PodsRunning, m.PodsTotal,
	)
	sev := SevWarn
	if m.PodHealthPct < 80 || m.ErrorRate > 15 {
		sev = SevError
	}
	return &Finding{
		Severity: sev,
		Title:    "Cluster metrics outside thresholds",
		Body:     body,
		Meta: map[string]any{
			"podHealthPct":  m.PodHealthPct,
			"errorRate":     m.ErrorRate,
			"warningEvents": m.WarningEvents,
			"podsTotal":     m.PodsTotal,
		},
	}, nil
}

// DecisionLogFreshnessCheck flags when DECISION_LOG.md has not been
// touched in a long time. The "documentation freshness" probe — if
// the team is actively shipping but the decision log is stale, that's
// a process drift to surface, not a cluster outage.
type DecisionLogFreshnessCheck struct {
	Path             string
	MaxStaleDuration time.Duration // default 30 days
}

func (DecisionLogFreshnessCheck) Name() string        { return "docs-freshness" }
func (DecisionLogFreshnessCheck) Description() string { return "Checking documentation freshness…" }

func (c DecisionLogFreshnessCheck) Run(ctx context.Context) (*Finding, error) {
	path := c.Path
	if path == "" {
		path = "DECISION_LOG.md"
	}
	max := c.MaxStaleDuration
	if max == 0 {
		max = 30 * 24 * time.Hour
	}
	stat, err := os.Stat(path)
	if err != nil {
		// Missing file — surface it as info, not error: the project
		// may simply not use a decision log.
		if os.IsNotExist(err) {
			return &Finding{
				Severity: SevInfo,
				Title:    "Decision log not found",
				Body:     fmt.Sprintf("`%s` does not exist. Consider keeping a running record of architectural decisions.", path),
				Meta:     map[string]any{"path": path},
			}, nil
		}
		return nil, err
	}
	age := time.Since(stat.ModTime())
	if age <= max {
		return nil, nil
	}
	days := int(age / (24 * time.Hour))
	return &Finding{
		Severity: SevWarn,
		Title:    fmt.Sprintf("Decision log stale (%d days)", days),
		Body:     fmt.Sprintf("`%s` has not been updated in **%d days**. If the team is shipping changes, the log should be too.", path, days),
		Meta: map[string]any{
			"path":     path,
			"ageDays":  days,
			"modTime":  stat.ModTime(),
			"thresholdDays": int(max / (24 * time.Hour)),
		},
	}, nil
}
