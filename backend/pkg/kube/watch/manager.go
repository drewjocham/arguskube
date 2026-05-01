package watch

import (
	"context"
	"log/slog"
	"time"

	"github.com/argues/kube-watcher/pkg/kube"
)

// Manager polls the cluster at a fixed interval and emits Alerts on a channel.
type Manager struct {
	client   kube.ClientInterface
	logger   *slog.Logger
	interval time.Duration
}

// NewManager creates a new watch Manager.
func NewManager(client kube.ClientInterface, logger *slog.Logger, interval time.Duration) *Manager {
	return &Manager{client: client, logger: logger, interval: interval}
}

// Start begins polling and returns a channel of alerts. The channel is closed
// when the context is cancelled.
func (m *Manager) Start(ctx context.Context) <-chan Alert {
	ch := make(chan Alert, 64)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		// Run immediately on start.
		m.poll(ctx, ch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.poll(ctx, ch)
			}
		}
	}()

	return ch
}

func (m *Manager) poll(ctx context.Context, ch chan<- Alert) {
	m.checkNodes(ctx, ch)
	m.checkPods(ctx, ch)
	m.checkEvents(ctx, ch)
}

func (m *Manager) checkNodes(ctx context.Context, ch chan<- Alert) {
	nodes, err := m.client.GetNodes(ctx)
	if err != nil {
		m.logger.Warn("watch: node check failed", slog.String("error", err.Error()))
		return
	}
	for _, n := range nodes {
		if n.Status != "Ready" {
			ch <- Alert{
				Kind:       AlertKindNode,
				Name:       n.Name,
				Severity:   "warning",
				Reason:     "NodeNotReady",
				Message:    n.Name + " is " + n.Status,
				OccurredAt: time.Now(),
			}
		}
	}
}

func (m *Manager) checkPods(ctx context.Context, ch chan<- Alert) {
	pods, err := m.client.GetPodsAllNamespaces(ctx)
	if err != nil {
		m.logger.Warn("watch: pod check failed", slog.String("error", err.Error()))
		return
	}
	for _, p := range pods {
		switch p.Status {
		case "CrashLoopBackOff", "Error", "ImagePullBackOff", "ErrImagePull":
			ch <- Alert{
				Kind:       AlertKindPod,
				Name:       p.Name,
				Namespace:  p.Namespace,
				Severity:   "warning",
				Reason:     p.Status,
				Message:    p.Name + " in " + p.Namespace + " is " + p.Status,
				OccurredAt: time.Now(),
			}
		}
		if p.RestartCount > 10 {
			ch <- Alert{
				Kind:       AlertKindPod,
				Name:       p.Name,
				Namespace:  p.Namespace,
				Severity:   "warning",
				Reason:     "HighRestarts",
				Message:    p.Name + " has high restart count",
				OccurredAt: time.Now(),
			}
		}
	}
}

func (m *Manager) checkEvents(ctx context.Context, ch chan<- Alert) {
	events, err := m.client.GetEventsAllNamespaces(ctx)
	if err != nil {
		m.logger.Warn("watch: event check failed", slog.String("error", err.Error()))
		return
	}
	cutoff := time.Now().Add(-m.interval * 2)
	for _, e := range events {
		if e.Type == "Warning" && e.LastTimestamp.After(cutoff) {
			ch <- Alert{
				Kind:       AlertKindEvent,
				Name:       e.ObjectName,
				Namespace:  "",
				Severity:   "info",
				Reason:     e.Reason,
				Message:    e.Message,
				OccurredAt: e.LastTimestamp,
			}
		}
	}
}
