package recommendation

import (
	"context"
	"fmt"
	"github.com/djocham/kube-watcher/mcp/monitoring/history"
	"log/slog"
	"time"

	"github.com/djocham/kube-watcher/pkg/kube/watch"
)

type Recommendation struct {
	Title          string            `json:"title"`
	Summary        string            `json:"summary"`
	Severity       string            `json:"severity"`
	Steps          []string          `json:"steps"`
	Evidence       map[string]string `json:"evidence"`
	FrequencyDelta float64           `json:"frequency_delta"`
	LastSeenCount  int               `json:"last_seen_count"`
}

type Engine struct {
	store  history.Recorder
	logger *slog.Logger
}

func NewEngine(store history.Recorder, logger *slog.Logger) *Engine {
	return &Engine{
		store:  store,
		logger: logger,
	}
}

func (e *Engine) ForAlert(ctx context.Context, alert watch.Alert) (Recommendation, error) {
	delta, err := e.store.CompareFrequency(ctx, history.IssueKind(alert.Kind), 6*time.Hour, 24*time.Hour)
	if err != nil {
		return Recommendation{}, err
	}

	evidence := map[string]string{
		"namespace": alert.Namespace,
		"name":      alert.Name,
		"reason":    alert.Reason,
		"message":   alert.Message,
	}

	rec := Recommendation{
		Title:          fmt.Sprintf("%s: %s", alert.Kind, alert.Severity),
		Summary:        alert.Message,
		Severity:       alert.Severity,
		Evidence:       evidence,
		Steps:          e.stepsForAlert(alert),
		FrequencyDelta: delta.PercentChange,
		LastSeenCount:  delta.RecentCount,
	}

	return rec, nil
}

func (e *Engine) stepsForAlert(alert watch.Alert) []string {
	switch alert.Kind {
	case watch.AlertKindNode:
		return []string{
			"Verify node connectivity and kubelet health.",
			"Check underlying infrastructure (disk pressure, memory, kube-proxy).",
			"Drain and cordon the node if recovery actions fail.",
		}
	case watch.AlertKindPod:
		return []string{
			"Inspect pod events for crash loop or image pull errors.",
			"Confirm resource requests/limits align with cluster capacity.",
			"Restart the workload or roll back the deployment if necessary.",
		}
	case watch.AlertKindEvent:
		return []string{
			"Review recent Warning events for issues.",
			"Audit cluster controllers emitting repeated warnings.",
		}
	default:
		return []string{"Review alert details for manual fix."}
	}
}
