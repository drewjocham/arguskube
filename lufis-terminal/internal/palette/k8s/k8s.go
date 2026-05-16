// Package k8s is the lufis command-palette implementation for the
// kubectl shell. It enumerates pods / services / configmaps / nodes
// via shelled-out `kubectl` calls (no client-go dep — lufis stays
// light) and renders Copy / Run command strings the popup buttons
// use.
//
// Kubeconfig + context come from the env Argus's LaunchPopOutTerminal
// inherits to lufis (see PR #82); we don't re-read them. The user's
// active KUBECONFIG / KUBECTX flow through naturally.
package k8s

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/argus/terminal/internal/palette"
)

const (
	shellName = "k8s"

	tabPods       = "pods"
	tabServices   = "services"
	tabConfigMaps = "configmaps"
	tabNodes      = "nodes"

	// collapseMin: with 1 replica there's nothing to fold; with 2 the
	// PODNAME-* form already saves screen space. K8s deployments
	// frequently hit dozens; collapsing aggressively pays off.
	collapseMin = 2
)

// Palette implements palette.ShellPalette for kubectl-style shells.
type Palette struct {
	// kubectlPath defaults to "kubectl" on $PATH. Overridable for
	// tests so we don't actually shell out.
	kubectlPath string

	// runner is what actually executes kubectl. Defaults to
	// exec.CommandContext; tests override with a recorded stub.
	runner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// New constructs a Palette using the kubectl on $PATH and the
// standard process runner.
func New() *Palette {
	return &Palette{
		kubectlPath: "kubectl",
		runner:      defaultRunner,
	}
}

// WithKubectl returns a Palette wired to a specific kubectl path.
// Used in tests and by callers that bundle their own kubectl binary.
func (p *Palette) WithKubectl(path string) *Palette {
	p.kubectlPath = path
	return p
}

// WithRunner injects the process-runner. Tests hand in a function
// that returns canned `kubectl get ... -o name` output.
func (p *Palette) WithRunner(r func(ctx context.Context, name string, args ...string) ([]byte, error)) *Palette {
	p.runner = r
	return p
}

func (p *Palette) Name() string { return shellName }

func (p *Palette) Tabs() []palette.Tab {
	return []palette.Tab{
		{ID: tabPods, Label: "Pods"},
		{ID: tabServices, Label: "Services"},
		{ID: tabConfigMaps, Label: "ConfigMaps"},
		{ID: tabNodes, Label: "Nodes"},
	}
}

func (p *Palette) List(ctx context.Context, tabID string) ([]palette.Group, error) {
	if _, ok := palette.FindTab(p.Tabs(), tabID); !ok {
		return nil, palette.ErrUnknownTab
	}
	args, namespaced := kubectlArgsForTab(tabID)
	out, err := p.runner(ctx, p.kubectlPath, args...)
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	names := parseKubectlNames(out)
	groups := palette.Collapse(names, collapseMin)
	if namespaced {
		// `kubectl get -A -o name` returns "pod/foo" lines without
		// the namespace — for the user-facing display we don't add
		// it back here. A future iteration can request -A with a
		// custom-columns format and stash the namespace in
		// Resource.Namespace.
		_ = namespaced
	}
	return groups, nil
}

func (p *Palette) Command(tab string, action palette.ActionID, picked palette.Resource) (string, error) {
	if _, ok := palette.FindTab(p.Tabs(), tab); !ok {
		return "", palette.ErrUnknownTab
	}
	if picked.Name == "" {
		return "", fmt.Errorf("k8s palette: picked resource is empty")
	}

	// Copy and Run-in-current produce the same command string; the
	// difference is what the render layer does with it (clipboard
	// vs PTY write). Same for Run-in-new-shell.
	switch action {
	case palette.ActionCopy, palette.ActionRunCurrent, palette.ActionRunNewShell:
		return commandForTab(tab, picked), nil
	default:
		return "", palette.ErrUnsupportedAction
	}
}

// commandForTab renders the default kubectl command for a tab's
// "drill down on a resource" action. The render layer's popup
// could later expose multiple actions per tab (logs / describe /
// exec for pods; describe / port-forward for services) — those would
// each map to a different ActionID.
func commandForTab(tab string, r palette.Resource) string {
	switch tab {
	case tabPods:
		// "kubectl describe pod NAME" is the most-asked-for next
		// step after picking a pod. Logs and exec are next, in a
		// follow-up that adds richer ActionIDs.
		return fmt.Sprintf("kubectl describe pod %s", r.Name)
	case tabServices:
		return fmt.Sprintf("kubectl describe svc %s", r.Name)
	case tabConfigMaps:
		return fmt.Sprintf("kubectl get cm %s -o yaml", r.Name)
	case tabNodes:
		return fmt.Sprintf("kubectl describe node %s", r.Name)
	default:
		return ""
	}
}

// kubectlArgsForTab returns the args + namespaced flag.
func kubectlArgsForTab(tab string) (args []string, namespaced bool) {
	switch tab {
	case tabPods:
		return []string{"get", "pods", "-A", "-o", "name"}, true
	case tabServices:
		return []string{"get", "services", "-A", "-o", "name"}, true
	case tabConfigMaps:
		return []string{"get", "configmaps", "-A", "-o", "name"}, true
	case tabNodes:
		return []string{"get", "nodes", "-o", "name"}, false
	default:
		return nil, false
	}
}

// parseKubectlNames splits the `-o name` output into bare names.
// kubectl emits one resource per line as "<kind>/<name>"; we strip
// the kind prefix because the palette already knows it from the tab.
func parseKubectlNames(out []byte) []string {
	lines := strings.Split(string(out), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if slash := strings.IndexByte(line, '/'); slash > 0 && slash < len(line)-1 {
			line = line[slash+1:]
		}
		names = append(names, line)
	}
	return names
}

// defaultRunner shells out to kubectl. Errors include kubectl's
// stderr so the user sees "the server doesn't have a resource type
// 'foo'" instead of a bare exit-status.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
