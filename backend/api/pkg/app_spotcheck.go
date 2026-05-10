package pkg

import (
	"fmt"
	"time"

	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/spotcheck"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// spotcheckNotifier adapts the spotcheck package's notification needs
// to Wails events. The frontend listens on:
//
//   - "argus:spotcheck:active"   — payload {checkName, description}.
//                                  Empty checkName means idle (hide pill).
//   - "argus:notification"       — payload mirrors the existing
//                                  notifications-store contract so the
//                                  bell + panel just work.
type spotcheckNotifier struct {
	app *App
}

func (n *spotcheckNotifier) Active(checkName, description string) {
	if n.app == nil || n.app.ctx == nil {
		return
	}
	runtime.EventsEmit(n.app.ctx, "argus:spotcheck:active", map[string]any{
		"checkName":   checkName,
		"description": description,
	})
}

func (n *spotcheckNotifier) Notify(checkName string, f spotcheck.Finding) {
	if n.app == nil || n.app.ctx == nil {
		return
	}
	// Map the spotcheck severity onto the kinds the frontend store
	// already understands. Anything more granular goes through `meta`.
	kind := "spot-check"
	switch f.Severity {
	case spotcheck.SevError:
		kind = "error"
	case spotcheck.SevWarn:
		kind = "warn"
	case spotcheck.SevInfo, spotcheck.SevOK:
		kind = "info"
	}
	runtime.EventsEmit(n.app.ctx, "argus:notification", map[string]any{
		"kind":         kind,
		"title":        f.Title,
		"body":         f.Body,
		"rerunnable":   true,
		"rerunPayload": map[string]string{"type": "spot-check", "name": checkName},
		"meta":         f.Meta,
	})
}

// CurrentMetrics implements spotcheck.MetricsProvider against App's
// cached snapshot. The cached value is refreshed on the existing
// metrics polling loop, so this never blocks on the K8s API.
func (a *App) CurrentMetrics() *alerts.ClusterMetrics {
	return a.cachedMetrics
}

// startSpotChecks builds the engine, registers the default checks,
// and kicks off the periodic loop. Safe to call without a k8s client —
// node/metrics checks degrade to "no cluster connected" findings.
func (a *App) startSpotChecks() {
	if a.spotcheck != nil {
		return
	}
	notifier := &spotcheckNotifier{app: a}

	interval := 30 * time.Minute
	a.spotcheck = spotcheck.New(a.logger, notifier, interval)

	if a.k8s != nil {
		a.spotcheck.Add(spotcheck.NodeReadyCheck{K8s: a.k8s})
	}
	a.spotcheck.Add(spotcheck.MetricsCheck{Source: a})
	a.spotcheck.Add(spotcheck.DecisionLogFreshnessCheck{
		Path:             a.cfg.DecisionLog.Path,
		MaxStaleDuration: 30 * 24 * time.Hour,
	})

	a.spotcheck.StartLoop(a.ctx)
}

// RunSpotChecks fires every registered check now, in a goroutine so
// the Wails RPC returns immediately. Bound to the frontend so the
// user can hit "run now" without waiting for the timer.
func (a *App) RunSpotChecks() error {
	if a.spotcheck == nil {
		return fmt.Errorf("spot-checks not initialized")
	}
	go a.spotcheck.RunAll(a.ctx)
	return nil
}

// RunSpotCheck fires a single check by name. The frontend's "rerun"
// button on a notification with rerunPayload.type === "spot-check"
// calls this with the check name.
func (a *App) RunSpotCheck(name string) error {
	if a.spotcheck == nil {
		return fmt.Errorf("spot-checks not initialized")
	}
	go func() {
		if err := a.spotcheck.RunOne(a.ctx, name); err != nil {
			a.logger.Warn("rerun failed",
				"check", name,
				"error", err.Error(),
			)
		}
	}()
	return nil
}

// ListSpotChecks returns the registered check names. Mostly diagnostic.
func (a *App) ListSpotChecks() []string {
	if a.spotcheck == nil {
		return nil
	}
	return a.spotcheck.Names()
}
