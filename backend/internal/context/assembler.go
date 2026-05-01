package context

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/apperrors"
	"github.com/argues/kube-watcher/internal/config"
	"github.com/argues/kube-watcher/internal/features"
)

// Assembler builds the bounded context bundle that feeds the AI diagnostics panel.
// Target: ~8k tokens, 3-second assembly window.
type Assembler struct {
	cfg      *config.OnlineDataConfig
	gate     *features.Gate
	detector anomaly.Detector
	logger   *slog.Logger
}

// NewAssembler creates a context assembler.
func NewAssembler(cfg *config.OnlineDataConfig, gate *features.Gate, detector anomaly.Detector, logger *slog.Logger) *Assembler {
	return &Assembler{
		cfg:      cfg,
		gate:     gate,
		detector: detector,
		logger:   logger,
	}
}

// Bundle is the assembled context passed to AI diagnostics.
type Bundle struct {
	Alert          alerts.Alert           `json:"alert"`
	DecisionLog    []DecisionEntry        `json:"decisionLog,omitempty"`
	CascadeAlerts  []alerts.Alert         `json:"cascadeAlerts,omitempty"`
	AnomalyResults []anomaly.DetectResult `json:"anomalyResults,omitempty"`
	Diagnosis      *alerts.Diagnosis      `json:"diagnosis,omitempty"`
}

// DecisionEntry is a parsed entry from DECISION_LOG.md.
type DecisionEntry struct {
	Date    string `json:"date"`
	Content string `json:"content"`
}

// Assemble builds a context bundle for a given alert within the configured timeout.
func (a *Assembler) Assemble(ctx context.Context, alert alerts.Alert, allAlerts []alerts.Alert) (*Bundle, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.AI.ContextTimeout)
	defer cancel()

	bundle := &Bundle{Alert: alert}

	type result struct {
		name string
		err  error
	}
	done := make(chan result, 3)

	// Parallel assembly within timeout window.

	// 1. Parse DECISION_LOG.md (pro only).
	go func() {
		if !a.gate.Allowed(features.FeatureDecisionLog) {
			done <- result{name: "decisionLog"}
			return
		}
		entries, err := a.parseDecisionLog(ctx, alert)
		if err != nil {
			a.logger.WarnContext(ctx, "decision log parse failed",
				slog.String("alertId", alert.ID),
				slog.String("error", err.Error()),
			)
			done <- result{name: "decisionLog", err: err}
			return
		}
		bundle.DecisionLog = entries
		done <- result{name: "decisionLog"}
	}()

	// 2. Cascade correlation (pro only).
	go func() {
		if !a.gate.Allowed(features.FeatureCascadeCorr) {
			done <- result{name: "cascade"}
			return
		}
		cascade := a.correlateCascade(alert, allAlerts)
		bundle.CascadeAlerts = cascade
		done <- result{name: "cascade"}
	}()

	// 3. Anomstack anomaly check (pro only).
	go func() {
		if !a.gate.Allowed(features.FeatureAnomstack) || a.detector == nil {
			done <- result{name: "anomaly"}
			return
		}
		res, err := a.detector.Detect(ctx, anomaly.DetectRequest{
			MetricName: metricNameForAlert(alert),
			Labels: map[string]string{
				"namespace": alert.Namespace,
				"pod":       alert.PodName,
				"node":      alert.NodeName,
			},
			Window: 30 * time.Minute,
		})
		if err != nil {
			a.logger.WarnContext(ctx, "anomstack detect failed",
				slog.String("alertId", alert.ID),
				slog.String("error", err.Error()),
			)
			done <- result{name: "anomaly", err: err}
			return
		}
		if res != nil {
			bundle.AnomalyResults = append(bundle.AnomalyResults, *res)
		}
		done <- result{name: "anomaly"}
	}()

	// Collect results, respect timeout.
	var errs []error
	for i := 0; i < 3; i++ {
		select {
		case r := <-done:
			if r.err != nil {
				errs = append(errs, r.err)
			}
		case <-ctx.Done():
			a.logger.WarnContext(ctx, "context assembly timeout",
				slog.String("alertId", alert.ID),
			)
			return bundle, apperrors.Mark(apperrors.ErrContextAssembly, apperrors.OK)
		}
	}

	if len(errs) > 0 {
		// Partial assembly is acceptable — return what we have.
		a.logger.WarnContext(ctx, "partial context assembly",
			slog.String("alertId", alert.ID),
			slog.Int("errors", len(errs)),
		)
	}

	// Generate diagnosis from the assembled context.
	bundle.Diagnosis = GenerateDiagnosis(bundle)

	return bundle, nil
}

// correlateCascade finds alerts that are causally related.
// e.g., DiskPressure → metrics-server eviction → HPA can't scale → CPU throttle.
func (a *Assembler) correlateCascade(target alerts.Alert, all []alerts.Alert) []alerts.Alert {
	var related []alerts.Alert

	for _, other := range all {
		if other.ID == target.ID {
			continue
		}

		// Same namespace correlation.
		if other.Namespace == target.Namespace {
			related = append(related, other)
			continue
		}

		// Node-to-pod cascade: node pressure can affect pods on that node.
		if target.NodeName != "" && other.NodeName == target.NodeName {
			related = append(related, other)
			continue
		}

		// Infra → workload cascade: evictions in infra affect dependent workloads.
		if other.Namespace == "infra" || other.Namespace == "monitoring" || other.Namespace == "kube-system" {
			for _, evicted := range other.EvictedPods {
				if isMonitoringComponent(evicted) {
					related = append(related, other)
					break
				}
			}
		}
	}

	return related
}

// parseDecisionLog reads DECISION_LOG.md and extracts entries relevant to the alert.
func (a *Assembler) parseDecisionLog(ctx context.Context, alert alerts.Alert) ([]DecisionEntry, error) {
	f, err := os.Open(a.cfg.DecisionLog.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No decision log — not an error.
		}
		return nil, errors.Join(apperrors.ErrDecisionLogParse, err)
	}
	defer f.Close()

	var entries []DecisionEntry
	var current *DecisionEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Date headers: "# 2026-04-10"
		if strings.HasPrefix(line, "# ") {
			if current != nil && isRelevant(current.Content, alert) {
				entries = append(entries, *current)
			}
			current = &DecisionEntry{
				Date:    strings.TrimPrefix(line, "# "),
				Content: "",
			}
			continue
		}

		if current != nil {
			current.Content += line + "\n"
		}
	}

	// Flush last entry.
	if current != nil && isRelevant(current.Content, alert) {
		entries = append(entries, *current)
	}

	return entries, scanner.Err()
}

// isRelevant checks if a decision log entry is relevant to the given alert.
func isRelevant(content string, alert alerts.Alert) bool {
	lower := strings.ToLower(content)

	// Match by pod name, namespace, node, or image tag.
	checks := []string{
		strings.ToLower(alert.PodName),
		strings.ToLower(alert.Namespace),
		strings.ToLower(alert.NodeName),
	}

	// Extract deployment base name (e.g., "payments-api" from "payments-api-7f9cd82b-x9kzp").
	if parts := strings.Split(alert.PodName, "-"); len(parts) >= 2 {
		checks = append(checks, strings.ToLower(strings.Join(parts[:2], "-")))
	}

	for _, check := range checks {
		if check != "" && strings.Contains(lower, check) {
			return true
		}
	}
	return false
}

func isMonitoringComponent(podName string) bool {
	monitoring := []string{"prometheus", "metrics-server", "alertmanager", "grafana", "loki", "thanos"}
	lower := strings.ToLower(podName)
	for _, m := range monitoring {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func metricNameForAlert(a alerts.Alert) string {
	switch {
	case strings.Contains(strings.ToLower(a.Name), "oom"):
		return fmt.Sprintf("container_memory_usage_bytes{namespace=%q,pod=%q}", a.Namespace, a.PodName)
	case strings.Contains(strings.ToLower(a.Name), "cpu"):
		return fmt.Sprintf("container_cpu_usage_seconds_total{namespace=%q,pod=%q}", a.Namespace, a.PodName)
	case strings.Contains(strings.ToLower(a.Name), "disk"):
		return fmt.Sprintf("node_filesystem_avail_bytes{node=%q}", a.NodeName)
	default:
		return fmt.Sprintf("up{namespace=%q,pod=%q}", a.Namespace, a.PodName)
	}
}
