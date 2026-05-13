package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ContextProbeResult is one row in the auto-resolver output: which contexts
// exist, which are reachable right now, how long each took, and which one
// the user (or a prior session) marked active. It's the data the settings
// checklist consumes to decide what to nudge the user about.
type ContextProbeResult struct {
	Name          string `json:"name"`
	Cluster       string `json:"cluster"`
	Active        bool   `json:"active"`
	Reachable     bool   `json:"reachable"`
	LatencyMs     int64  `json:"latencyMs"`
	ServerVersion string `json:"serverVersion,omitempty"`
	Error         string `json:"error,omitempty"`
}

// ContextResolution is the outcome of the auto-resolve flow. Chosen is the
// context the app should bind to right now (active-if-reachable > first-
// reachable > active-anyway-flagged). Confidence describes why we picked it
// so the UI can render an honest message.
type ContextResolution struct {
	Chosen        string               `json:"chosen"`
	Confidence    string               `json:"confidence"` // "active-reachable" | "fallback-reachable" | "active-unreachable" | "none"
	Probes        []ContextProbeResult `json:"probes"`
	ReachableCount int                 `json:"reachableCount"`
}

// versionProber abstracts the per-context reachability probe so tests can
// inject a fake without spinning up real clusters. The real implementation
// builds a kubernetes.Interface from the rest.Config and calls
// Discovery().ServerVersion(); see realProbe below.
type versionProber func(ctx context.Context, restCfg *rest.Config) (string, error)

// defaultProbe is the production version-probe — a Discovery call against
// the API server. It's behind a variable so tests can substitute it.
var defaultProbe versionProber = func(ctx context.Context, restCfg *rest.Config) (string, error) {
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return "", err
	}
	return probeServerVersion(ctx, cs.Discovery())
}

func probeServerVersion(ctx context.Context, d discovery.DiscoveryInterface) (string, error) {
	// Discovery doesn't accept a context directly; we honour the deadline by
	// running the call on a goroutine and racing it against ctx.Done().
	type vr struct {
		v   string
		err error
	}
	ch := make(chan vr, 1)
	go func() {
		info, err := d.ServerVersion()
		if err != nil {
			ch <- vr{err: err}
			return
		}
		ch <- vr{v: info.GitVersion}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// ProbeContexts reads kubeconfig (multi-file aware via
// kubeconfigLoadingRules) and fans out a reachability probe against each
// context in parallel. Each probe is bounded by timeout. The function never
// returns an error for a single failed probe — the failure goes into the
// per-row Error field — but it does return an error if kubeconfig itself
// can't be loaded.
//
// Results are sorted active-first, then alphabetical. Callers can pass
// activeOverride to mark a different context than kubeconfig's
// current-context as active (e.g. one persisted from a prior session).
func ProbeContexts(ctx context.Context, kubeconfigPath, activeOverride string, timeout time.Duration) ([]ContextProbeResult, error) {
	rules := kubeconfigLoadingRules(kubeconfigPath)
	rawCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	activeCtx := activeOverride
	if activeCtx == "" {
		activeCtx = rawCfg.CurrentContext
	}

	names := make([]string, 0, len(rawCfg.Contexts))
	for name := range rawCfg.Contexts {
		names = append(names, name)
	}

	results := make([]ContextProbeResult, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			cluster := ""
			if ctxEntry, ok := rawCfg.Contexts[name]; ok && ctxEntry != nil {
				cluster = ctxEntry.Cluster
			}
			row := ContextProbeResult{
				Name:    name,
				Cluster: cluster,
				Active:  name == activeCtx,
			}

			overrides := &clientcmd.ConfigOverrides{CurrentContext: name}
			restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
			if err != nil {
				row.Error = err.Error()
				results[i] = row
				return
			}

			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			start := time.Now()
			version, err := defaultProbe(probeCtx, restCfg)
			row.LatencyMs = time.Since(start).Milliseconds()
			if err != nil {
				row.Error = err.Error()
				row.Reachable = false
			} else {
				row.Reachable = true
				row.ServerVersion = version
			}
			results[i] = row
		}(i, name)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Active != results[j].Active {
			return results[i].Active
		}
		return results[i].Name < results[j].Name
	})
	return results, nil
}

// ChooseContext applies the auto-resolver priorities to a probe set:
//  1. active context, if reachable
//  2. otherwise the first reachable context (alphabetical, since sorted)
//  3. otherwise the active context anyway, flagged "active-unreachable"
//  4. otherwise empty + "none" when kubeconfig has no contexts at all
//
// The choice is data, not policy — the caller decides whether to actually
// SwitchContext() to it. We return the reason string so the UI can render
// an honest message instead of a generic "connected".
func ChooseContext(probes []ContextProbeResult) ContextResolution {
	res := ContextResolution{Probes: probes}
	if len(probes) == 0 {
		res.Confidence = "none"
		return res
	}

	for _, p := range probes {
		if p.Reachable {
			res.ReachableCount++
		}
	}

	var active *ContextProbeResult
	for i := range probes {
		if probes[i].Active {
			active = &probes[i]
			break
		}
	}

	if active != nil && active.Reachable {
		res.Chosen = active.Name
		res.Confidence = "active-reachable"
		return res
	}

	for _, p := range probes {
		if p.Reachable {
			res.Chosen = p.Name
			res.Confidence = "fallback-reachable"
			return res
		}
	}

	if active != nil {
		res.Chosen = active.Name
		res.Confidence = "active-unreachable"
		return res
	}

	res.Chosen = probes[0].Name
	res.Confidence = "active-unreachable"
	return res
}

// ErrNoContexts is returned when kubeconfig has zero contexts. Callers
// should treat this as a checklist item ("Argus didn't find any
// kubeconfig contexts — [Open kubeconfig docs]") rather than a hard fail.
var ErrNoContexts = errors.New("kubeconfig has no contexts")
