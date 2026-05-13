package pkg

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/argues/argus/internal/envprobe"
)

// envProbeBundle holds the runner and a one-time-init guard. We keep it
// off the App struct to avoid bloating the god-object further (already
// flagged as Critical Debt in .context.md) — the singleton lives at file
// scope, keyed by App pointer in case a future test spins up multiple.
type envProbeBundle struct {
	runner *envprobe.Runner
}

var (
	envProbeMu sync.Mutex
	envProbes  = map[*App]*envProbeBundle{}
)

const envProbePeriod = 60 * time.Second

// initEnvProbes lazily wires the runner the first time AutoResolveContext
// has produced a real API server URL. Constructed lazily so the probes
// always observe the current context — switching clusters does not need
// re-wiring because the APIHostProvider closes over a *App method.
func (a *App) initEnvProbes() *envprobe.Runner {
	envProbeMu.Lock()
	defer envProbeMu.Unlock()
	if b, ok := envProbes[a]; ok {
		return b.runner
	}
	hostProvider := func() string {
		if a.k8s == nil {
			return ""
		}
		cfg := a.k8s.GetRestConfig()
		if cfg == nil {
			return ""
		}
		return cfg.Host
	}
	// Lazy clientset provider: re-read on every probe so a SwitchContext
	// is observed without re-wiring. Returns nil when no cluster is bound,
	// which the probe degrades to a soft Warn for.
	clientsetProvider := func() kubernetes.Interface {
		if a.k8s == nil {
			return nil
		}
		return a.k8s.GetClientset()
	}
	probes := []envprobe.Probe{
		envprobe.NewDNSProbe(hostProvider, nil),
		envprobe.NewTLSChainProbe(hostProvider, nil),
		envprobe.NewClockSkewProbe(hostProvider, nil, 30*time.Second),
		envprobe.NewSignedImagesProbeFromClient(clientsetProvider),
	}
	r := envprobe.NewRunner(a.logger.With("component", "envprobe"), 3*time.Second, probes...)
	envProbes[a] = &envProbeBundle{runner: r}
	return r
}

// RunEnvProbes runs every registered environment probe in parallel and
// returns the results. Each probe also publishes a StatusEvent so the
// bottom ribbon narrates the sweep, and a typed event so the frontend
// checklist producer turns the results into rows.
//
// Safe to call repeatedly — the runner caches the latest results so
// re-runs do not double-emit if nothing changed (the frontend producer
// upserts by id). The signed-images probe is added lazily once the K8s
// client is bound (which happens after AutoResolveContext picks a
// reachable context).
func (a *App) RunEnvProbes() ([]envprobe.Result, error) {
	runner := a.initEnvProbes()

	a.emitStatus("envprobe", "info", "Checking environment…", "")
	results := runner.RunAll(a.ctx)

	for _, res := range results {
		a.publishEnvProbeResult(res)
	}
	a.emitStatus("envprobe", "info", summarize(results), "")
	return results, nil
}

// StartEnvProbeLoop fires the probes once and then on a 60s ticker. The
// initial run gives the user immediate feedback after sign-in; the
// periodic re-run picks up VPN-off / firewall changes during the
// session. Called from Startup.
func (a *App) StartEnvProbeLoop(ctx context.Context) {
	go func() {
		// First sweep at startup. We let any other Startup work settle
		// for a beat so the kubeconfig auto-resolver has a chance to
		// pick a context before TLS/DNS probes target a stale host.
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return
		}
		_, _ = a.RunEnvProbes()

		ticker := time.NewTicker(envProbePeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if a.paused.Load() {
					continue
				}
				_, _ = a.RunEnvProbes()
			}
		}
	}()
}

func (a *App) publishEnvProbeResult(res envprobe.Result) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "argus:envprobe", map[string]any{
		"id":          res.ID,
		"title":       res.Title,
		"status":      string(res.Status),
		"detail":      res.Detail,
		"actionLabel": res.ActionLabel,
		"actionId":    res.ActionID,
		"latencyMs":   res.Latency.Milliseconds(),
	})
	// One status-ribbon line per non-OK probe. We skip OK results so the
	// ribbon stays calm — successful checks live in the checklist, not
	// the marquee.
	if res.Status == envprobe.OK {
		return
	}
	sev := "info"
	switch res.Status {
	case envprobe.Warn:
		sev = "warn"
	case envprobe.Todo, envprobe.Error:
		sev = "warn"
	}
	a.emitStatus("envprobe", sev, res.Title, res.Detail)
}

func summarize(results []envprobe.Result) string {
	ok, warn, todo, err := 0, 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case envprobe.OK:
			ok++
		case envprobe.Warn:
			warn++
		case envprobe.Todo:
			todo++
		case envprobe.Error:
			err++
		}
	}
	if warn == 0 && todo == 0 && err == 0 {
		return "Environment looks good"
	}
	parts := ""
	if todo > 0 {
		parts += fmtCount(todo, "needs action")
	}
	if warn > 0 {
		if parts != "" {
			parts += ", "
		}
		parts += fmtCount(warn, "warning")
	}
	if err > 0 {
		if parts != "" {
			parts += ", "
		}
		parts += fmtCount(err, "blocker")
	}
	return "Environment: " + parts
}

func fmtCount(n int, label string) string {
	suffix := ""
	if n != 1 {
		suffix = "s"
	}
	return strconv.Itoa(n) + " " + label + suffix
}
