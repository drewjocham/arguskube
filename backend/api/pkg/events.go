package pkg

import (
	"bufio"
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/alerts"
)

const (
	EventAlertUpdate   = "alert:update"
	EventLogLine       = "log:line"
	EventMetricsUpdate = "metrics:update"
	EventAutoSummary   = "agent:auto-summary"
	EventAgentEvent    = "agent:event"
)

// StartEventLoop begins background polling for alerts, metrics, and logs,
// pushing updates to the Vue frontend via Wails EventsEmit.
func (a *App) StartEventLoop(ctx context.Context) {
	if a.k8s == nil {
		a.logger.Warn("event loop not started — no cluster connection")
		return
	}
	go a.pollAlerts(ctx)
	go a.pollMetrics(ctx)
	go a.pollLogs(ctx)
}

func (a *App) pollAlerts(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track seen alert IDs to trigger auto-investigation only for new alerts.
	seen := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.paused.Load() {
				continue
			}
			alertList, err := a.k8s.DetectAlerts(ctx)
			if err != nil {
				a.logger.WarnContext(ctx, "alert poll failed",
					slog.String("error", err.Error()),
				)
				continue
			}
			// Merge webhook-received alerts into the live stream.
			a.webhookMu.RLock()
			alertList = append(alertList, a.webhookAlerts...)
			a.webhookMu.RUnlock()

			// Run through the alert processor: deduplicates by signature,
			// suppresses silenced groups, kicks off agent investigations
			// (when the user's profile permits), and emits the fatigue
			// meta-alert when noise crosses threshold. The frontend only
			// sees the FILTERED list — never the spam of repeated firings.
			alertList = a.processAlertsThroughAgent(alertList)

			runtime.EventsEmit(ctx, EventAlertUpdate, alertList)

			// Auto-investigate new alerts.
			for i := range alertList {
				alert := alertList[i]

				if !seen[alert.ID] {
					seen[alert.ID] = true

					// Track the event.
					if a.agent != nil {
						a.agent.TrackEvent(ai.AgentEvent{
							Type:      "alert",
							Summary:   alert.Name + " — " + alert.Description,
							AlertID:   alert.ID,
							Namespace: alert.Namespace,
							Severity:  string(alert.Severity),
						})

						// Trigger auto-investigation (non-blocking).
						a.agent.AutoInvestigate(ctx, alert, a.cachedMetrics, alertList)
					}
				}

				// Emit log lines for each alert's pod.
				if alert.PodName == "" {
					continue
				}
				lines, err := a.k8s.GetPodLogs(ctx, alert.Namespace, alert.PodName, 5)
				if err != nil {
					continue
				}
				for _, line := range lines {
					runtime.EventsEmit(ctx, EventLogLine, line)
				}
			}

			// Prune seen alerts that have resolved.
			activeIDs := make(map[string]bool, len(alertList))
			for _, alert := range alertList {
				activeIDs[alert.ID] = true
			}
			for id := range seen {
				if !activeIDs[id] {
					delete(seen, id)
					if a.agent != nil {
						a.agent.TrackEvent(ai.AgentEvent{
							Type:    "resolution",
							Summary: "Alert resolved: " + id,
							AlertID: id,
						})
					}
				}
			}
		}
	}
}

func (a *App) pollMetrics(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.paused.Load() {
				continue
			}
			m, err := a.k8s.GetMetrics(ctx)
			if err != nil {
				continue
			}
			a.cachedMetrics = m
			runtime.EventsEmit(ctx, EventMetricsUpdate, m)
		}
	}
}

// EmitLogLine allows manual log injection (for testing/demo).
func (a *App) EmitLogLine(line alerts.LogLine) {
	runtime.EventsEmit(a.ctx, EventLogLine, line)
}

func (a *App) pollLogs(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	firstRun := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if a.paused.Load() {
				continue
			}
			a.emitRecentLogs(ctx, firstRun)
			firstRun = false
		}
	}
}

// emitRecentLogs fetches recent log lines from running pods and emits them.
// On the first call it fetches the last 50 lines per container for an initial
// seed; subsequent calls use SinceTime to get only new lines.
func (a *App) emitRecentLogs(ctx context.Context, seed bool) {
	ns := ""
	if a.cfg != nil {
		ns = a.cfg.Kubernetes.Namespace
	}

	pods, err := a.k8s.GetClientset().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		a.logger.WarnContext(ctx, "log poll: list pods failed", slog.String("error", err.Error()))
		return
	}

	now := time.Now()
	emitted := 0
	const maxLines = 200 // cap total lines per tick

	for i := range pods.Items {
		if emitted >= maxLines {
			break
		}
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodRunning {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			if emitted >= maxLines {
				break
			}
			opts := &corev1.PodLogOptions{
				Container: cs.Name,
			}
			if seed {
				// First run: grab last 50 lines per container for initial display.
				var tail int64 = 50
				opts.TailLines = &tail
			} else {
				// Subsequent runs: only new lines since last poll.
				since := metav1.NewTime(now.Add(-4 * time.Second))
				opts.SinceTime = &since
			}

			req := a.k8s.GetClientset().CoreV1().Pods(p.Namespace).GetLogs(p.Name, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) == 0 {
					continue
				}

				lowerMsg := strings.ToLower(line)
				level := "info"
				if strings.Contains(lowerMsg, "error") || strings.Contains(lowerMsg, "fail") || strings.Contains(lowerMsg, "fatal") {
					level = "error"
				} else if strings.Contains(lowerMsg, "warn") {
					level = "warn"
				}

				entry := alerts.LogLine{
					Timestamp: now,
					Source:    "[" + p.Name + "]",
					Level:     level,
					Message:   line,
				}
				runtime.EventsEmit(ctx, EventLogLine, entry)
				emitted++
				if emitted >= maxLines {
					break
				}
			}
			stream.Close()
		}
	}
}
