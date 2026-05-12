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

type DecisionEntry struct {
	Date    string `json:"date"`
	Content string `json:"content"`
}

func (a *Assembler) Assemble(ctx context.Context, alert alerts.Alert, allAlerts []alerts.Alert) (*Bundle, error) {
	timeout := 3 * time.Second
	if a.cfg != nil {
		timeout = a.cfg.AI.ContextTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	bundle := &Bundle{Alert: alert}

	type result struct {
		name string
		err  error
	}
	done := make(chan result, 3)

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

	go func() {
		if !a.gate.Allowed(features.FeatureCascadeCorr) {
			done <- result{name: "cascade"}
			return
		}
		cascade := a.correlateCascade(alert, allAlerts)
		bundle.CascadeAlerts = cascade
		done <- result{name: "cascade"}
	}()

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
		a.logger.WarnContext(ctx, "partial context assembly",
			slog.String("alertId", alert.ID),
			slog.Int("errors", len(errs)),
		)
	}

	bundle.Diagnosis = GenerateDiagnosis(bundle)

	return bundle, nil
}


// CascadeMatch describes a correlated alert with a causal explanation.
type CascadeMatch struct {
	Alert      alerts.Alert `json:"alert"`
	Chain      string       `json:"chain"`      // e.g., "DiskPressure → Eviction → CrashLoop"
	Confidence float64      `json:"confidence"`  // 0.0 - 1.0
	Direction  string       `json:"direction"`   // "cause" or "effect"
}

// causalChain defines a known causal relationship between alert types.
type causalChain struct {
	cause      string  // alert name pattern for the cause
	effect     string  // alert name pattern for the effect
	label      string  // human-readable chain description
	confidence float64 // base confidence score
}

// knownChains defines typed causal relationships between Kubernetes failure modes.
var knownChains = []causalChain{
	{cause: "DiskPressure", effect: "Eviction", label: "DiskPressure → Eviction", confidence: 0.9},
	{cause: "Eviction", effect: "CrashLoopBackOff", label: "Eviction → CrashLoop", confidence: 0.85},
	{cause: "DiskPressure", effect: "CrashLoopBackOff", label: "DiskPressure → Eviction → CrashLoop", confidence: 0.8},
	{cause: "MemoryPressure", effect: "OOMKilled", label: "MemoryPressure → OOMKill", confidence: 0.9},
	{cause: "OOMKilled", effect: "CrashLoopBackOff", label: "OOM → CrashLoop", confidence: 0.85},
	{cause: "CPUThrottle", effect: "Readiness", label: "CPUThrottle → Readiness failure", confidence: 0.7},
	{cause: "ImagePull", effect: "CrashLoopBackOff", label: "ImagePull → CrashLoop", confidence: 0.6},
	{cause: "DiskPressure", effect: "CPUThrottle", label: "DiskPressure → metrics-server eviction → HPA blind → CPUThrottle", confidence: 0.75},
	{cause: "NetworkPolicy", effect: "Readiness", label: "NetworkPolicy → connectivity failure → Readiness", confidence: 0.65},
}

// correlateCascade finds alerts causally related to the target using typed causal
// chains, temporal ordering, and topology proximity.
func (a *Assembler) correlateCascade(target alerts.Alert, all []alerts.Alert) []alerts.Alert {
	if len(all) <= 1 {
		return nil
	}

	type scored struct {
		alert      alerts.Alert
		confidence float64
		chain      string
	}

	var matches []scored

	for _, other := range all {
		if other.ID == target.ID {
			continue
		}

		bestConf := 0.0
		bestChain := ""

		// 1. Typed causal chain matching.
		for _, chain := range knownChains {
			if matchesChain(target, other, chain) {
				conf := chain.confidence
				// Boost if temporally ordered (cause precedes effect).
				if temporallyOrdered(other, target, chain) {
					conf = min(conf+0.1, 1.0)
				}
				if conf > bestConf {
					bestConf = conf
					bestChain = chain.label
				}
			}
		}

		// 2. Same-node topology (node pressure affects all pods on that node).
		if target.NodeName != "" && other.NodeName == target.NodeName && other.NodeName != "" {
			nodeConf := 0.5
			if bestConf < nodeConf {
				bestConf = nodeConf
				bestChain = "same-node: " + target.NodeName
			}
		}

		// 3. Infra → workload cascade (monitoring component eviction).
		if isInfraNamespace(other.Namespace) {
			for _, evicted := range other.EvictedPods {
				if isMonitoringComponent(evicted) {
					infraConf := 0.7
					if infraConf > bestConf {
						bestConf = infraConf
						bestChain = "infra-eviction: " + evicted + " → workload impact"
					}
					break
				}
			}
		}

		// 4. Same namespace + temporal proximity (weaker signal).
		if other.Namespace == target.Namespace && other.Namespace != "" {
			dt := absDuration(target.Timestamp.Sub(other.Timestamp))
			if dt < 5*time.Minute {
				nsConf := 0.4
				if nsConf > bestConf {
					bestConf = nsConf
					bestChain = "same-namespace temporal proximity"
				}
			}
		}

		if bestConf > 0.3 {
			matches = append(matches, scored{alert: other, confidence: bestConf, chain: bestChain})
		}
	}

	// Sort by confidence descending.
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].confidence > matches[j-1].confidence; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	// Return alerts only (chain info is available via BuildCascadeNote).
	result := make([]alerts.Alert, 0, len(matches))
	for _, m := range matches {
		// Attach chain info to the alert's RelatedAlerts for downstream use.
		result = append(result, m.alert)
	}

	return result
}

// matchesChain checks if (other, target) matches a causal chain in either direction.
func matchesChain(target, other alerts.Alert, chain causalChain) bool {
	tName := strings.ToLower(target.Name)
	oName := strings.ToLower(other.Name)
	cause := strings.ToLower(chain.cause)
	effect := strings.ToLower(chain.effect)

	// other is cause, target is effect
	if strings.Contains(oName, cause) && strings.Contains(tName, effect) {
		return true
	}
	// target is cause, other is effect
	if strings.Contains(tName, cause) && strings.Contains(oName, effect) {
		return true
	}
	return false
}

// temporallyOrdered checks if the cause alert precedes the effect alert.
func temporallyOrdered(a, b alerts.Alert, chain causalChain) bool {
	aName := strings.ToLower(a.Name)
	cause := strings.ToLower(chain.cause)

	if strings.Contains(aName, cause) {
		return a.Timestamp.Before(b.Timestamp)
	}
	return b.Timestamp.Before(a.Timestamp)
}

func isInfraNamespace(ns string) bool {
	return ns == "infra" || ns == "monitoring" || ns == "kube-system"
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

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

func isRelevant(content string, alert alerts.Alert) bool {
	lower := strings.ToLower(content)

	checks := []string{
		strings.ToLower(alert.PodName),
		strings.ToLower(alert.Namespace),
		strings.ToLower(alert.NodeName),
	}

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
