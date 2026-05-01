package pkg

import (
	"context"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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
			alertList, err := a.k8s.DetectAlerts(ctx)
			if err != nil {
				a.logger.WarnContext(ctx, "alert poll failed",
					slog.String("error", err.Error()),
				)
				continue
			}
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
