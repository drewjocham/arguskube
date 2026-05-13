package k8s

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/rest"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: prod
clusters:
- name: prod-cluster
  cluster:
    server: https://prod.example:6443
- name: stage-cluster
  cluster:
    server: https://stage.example:6443
- name: dev-cluster
  cluster:
    server: https://dev.example:6443
contexts:
- name: prod
  context:
    cluster: prod-cluster
    user: alice
- name: stage
  context:
    cluster: stage-cluster
    user: alice
- name: dev
  context:
    cluster: dev-cluster
    user: alice
users:
- name: alice
  user:
    token: stub
`

// writeKubeconfig writes a temporary kubeconfig and returns its path.
func writeKubeconfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return p
}

// stubProber replaces defaultProbe for one test. Returns a restore fn so
// other tests in this file aren't affected.
func stubProber(t *testing.T, fn versionProber) {
	t.Helper()
	prev := defaultProbe
	defaultProbe = fn
	t.Cleanup(func() { defaultProbe = prev })
}

func TestProbeContexts_AllReachable(t *testing.T) {
	p := writeKubeconfig(t, testKubeconfig)
	stubProber(t, func(ctx context.Context, restCfg *rest.Config) (string, error) {
		// Every server resolves; pretend the dev cluster is on a newer
		// version so we can verify ServerVersion threads through.
		if strings.Contains(restCfg.Host, "dev") {
			return "v1.30.2", nil
		}
		return "v1.29.5", nil
	})

	res, err := ProbeContexts(context.Background(), p, "", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("want 3 probes, got %d", len(res))
	}
	// Active-first sort puts prod at index 0.
	if !res[0].Active || res[0].Name != "prod" {
		t.Fatalf("active sort broken: %+v", res[0])
	}
	for _, r := range res {
		if !r.Reachable {
			t.Errorf("expected reachable for %s, got %s", r.Name, r.Error)
		}
		if r.ServerVersion == "" {
			t.Errorf("missing serverVersion for %s", r.Name)
		}
	}
}

func TestProbeContexts_SomeUnreachable(t *testing.T) {
	p := writeKubeconfig(t, testKubeconfig)
	stubProber(t, func(ctx context.Context, restCfg *rest.Config) (string, error) {
		if strings.Contains(restCfg.Host, "stage") {
			return "", errors.New("dial tcp: i/o timeout")
		}
		return "v1.29.5", nil
	})

	res, err := ProbeContexts(context.Background(), p, "", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	byName := map[string]ContextProbeResult{}
	for _, r := range res {
		byName[r.Name] = r
	}
	if byName["stage"].Reachable {
		t.Fatalf("stage should be unreachable, got %+v", byName["stage"])
	}
	if byName["stage"].Error == "" {
		t.Errorf("expected error string on unreachable probe")
	}
	if !byName["prod"].Reachable {
		t.Errorf("prod should be reachable, got %+v", byName["prod"])
	}
}

func TestProbeContexts_HonorsTimeout(t *testing.T) {
	p := writeKubeconfig(t, testKubeconfig)
	stubProber(t, func(ctx context.Context, restCfg *rest.Config) (string, error) {
		// Hang forever, force the per-context timeout to expire.
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			return "v1", nil
		}
	})

	start := time.Now()
	res, err := ProbeContexts(context.Background(), p, "", 50*time.Millisecond)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("probes did not respect timeout, took %v", elapsed)
	}
	for _, r := range res {
		if r.Reachable {
			t.Errorf("context %s should not be reachable under tight timeout", r.Name)
		}
	}
}

func TestProbeContexts_ActiveOverride(t *testing.T) {
	p := writeKubeconfig(t, testKubeconfig)
	stubProber(t, func(ctx context.Context, restCfg *rest.Config) (string, error) {
		return "v1.29.5", nil
	})

	res, err := ProbeContexts(context.Background(), p, "stage", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	// Active flag honours the override, not the file's current-context.
	for _, r := range res {
		if r.Name == "stage" && !r.Active {
			t.Errorf("activeOverride did not promote stage")
		}
		if r.Name == "prod" && r.Active {
			t.Errorf("activeOverride did not demote prod")
		}
	}
}

func TestProbeContexts_NoKubeconfig(t *testing.T) {
	_, err := ProbeContexts(context.Background(), "/no/such/path/kubeconfig", "", 50*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error for missing kubeconfig")
	}
}

func TestChooseContext_ActiveReachable(t *testing.T) {
	probes := []ContextProbeResult{
		{Name: "prod", Active: true, Reachable: true},
		{Name: "stage", Reachable: true},
		{Name: "dev", Reachable: false},
	}
	r := ChooseContext(probes)
	if r.Chosen != "prod" {
		t.Errorf("want prod, got %s", r.Chosen)
	}
	if r.Confidence != "active-reachable" {
		t.Errorf("want active-reachable, got %s", r.Confidence)
	}
	if r.ReachableCount != 2 {
		t.Errorf("want 2 reachable, got %d", r.ReachableCount)
	}
}

func TestChooseContext_FallbackToReachable(t *testing.T) {
	probes := []ContextProbeResult{
		{Name: "prod", Active: true, Reachable: false, Error: "i/o timeout"},
		{Name: "dev", Reachable: true},
		{Name: "stage", Reachable: true},
	}
	r := ChooseContext(probes)
	if r.Chosen != "dev" {
		t.Errorf("want first reachable (dev), got %s", r.Chosen)
	}
	if r.Confidence != "fallback-reachable" {
		t.Errorf("want fallback-reachable, got %s", r.Confidence)
	}
}

func TestChooseContext_NoneReachable(t *testing.T) {
	probes := []ContextProbeResult{
		{Name: "prod", Active: true, Reachable: false},
		{Name: "stage", Reachable: false},
	}
	r := ChooseContext(probes)
	if r.Chosen != "prod" {
		t.Errorf("want active fallback, got %s", r.Chosen)
	}
	if r.Confidence != "active-unreachable" {
		t.Errorf("want active-unreachable, got %s", r.Confidence)
	}
	if r.ReachableCount != 0 {
		t.Errorf("want 0 reachable, got %d", r.ReachableCount)
	}
}

func TestChooseContext_NoActiveAndNoneReachable(t *testing.T) {
	probes := []ContextProbeResult{
		{Name: "alpha", Reachable: false},
		{Name: "beta", Reachable: false},
	}
	r := ChooseContext(probes)
	if r.Chosen != "alpha" {
		t.Errorf("want first context as fallback, got %s", r.Chosen)
	}
	if r.Confidence != "active-unreachable" {
		t.Errorf("want active-unreachable, got %s", r.Confidence)
	}
}

func TestChooseContext_Empty(t *testing.T) {
	r := ChooseContext(nil)
	if r.Chosen != "" || r.Confidence != "none" {
		t.Errorf("want empty/none, got %+v", r)
	}
}
