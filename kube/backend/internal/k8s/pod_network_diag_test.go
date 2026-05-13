package k8s

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// newDiagClient builds a Client backed by a fake clientset for diag tests.
// The returned *fake.Clientset lets callers seed objects before each test.
func newDiagClient(cs *fake.Clientset) *Client {
	return &Client{
		cs:     cs,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// fakeExec returns a runExec stub that always returns the given output/error.
func fakeExec(out string, err error) func(ctx context.Context, ns, pod, container string, cmd []string) (string, error) {
	return func(ctx context.Context, ns, pod, container string, cmd []string) (string, error) {
		return out, err
	}
}

// seqExec returns a stub that yields the given (out, err) tuples in order.
// RunPodDNSCheck now performs TWO exec calls — a tool-detection probe and
// then the actual lookup — so single-shot fakeExec can't cover the happy
// path realistically. The sequenced helper threads the tool name into the
// first call and the lookup output into the second, matching how a real
// container would respond. After the last entry runs, further calls
// return the LAST tuple (so tests don't have to enumerate every call).
func seqExec(steps ...struct {
	out string
	err error
}) func(ctx context.Context, ns, pod, container string, cmd []string) (string, error) {
	i := 0
	return func(ctx context.Context, ns, pod, container string, cmd []string) (string, error) {
		s := steps[i]
		if i < len(steps)-1 {
			i++
		}
		return s.out, s.err
	}
}

// --- helpers ---

func makeDiagPod(ns, name string, mutators ...func(*corev1.Pod)) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
			},
		},
		Status: corev1.PodStatus{
			PodIP:  "10.0.0.1",
			HostIP: "192.168.1.1",
		},
	}
	for _, m := range mutators {
		m(p)
	}
	return p
}

func makeDaemonSet(ns, name string, desired, ready int32) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberReady:            ready,
		},
	}
}

// ============================================================
// TestGetPodNetworkInfo
// ============================================================

func TestGetPodNetworkInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("basic pod IP and HostIP", func(t *testing.T) {
		cs := fake.NewSimpleClientset(makeDiagPod("default", "my-pod"))
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "my-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.PodIP != "10.0.0.1" {
			t.Errorf("PodIP = %q, want 10.0.0.1", info.PodIP)
		}
		if info.HostIP != "192.168.1.1" {
			t.Errorf("HostIP = %q, want 192.168.1.1", info.HostIP)
		}
	})

	t.Run("HostNetwork false by default", func(t *testing.T) {
		cs := fake.NewSimpleClientset(makeDiagPod("default", "my-pod"))
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "my-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.HostNetwork {
			t.Error("expected HostNetwork=false")
		}
	})

	t.Run("HostNetwork true", func(t *testing.T) {
		p := makeDiagPod("default", "host-pod", func(p *corev1.Pod) {
			p.Spec.HostNetwork = true
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "host-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.HostNetwork {
			t.Error("expected HostNetwork=true")
		}
	})

	t.Run("container list populated", func(t *testing.T) {
		p := makeDiagPod("default", "multi-ctr", func(p *corev1.Pod) {
			p.Spec.Containers = []corev1.Container{
				{Name: "web"},
				{Name: "sidecar"},
			}
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "multi-ctr")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.Containers) != 2 {
			t.Errorf("containers len = %d, want 2", len(info.Containers))
		}
		if info.Containers[0] != "web" || info.Containers[1] != "sidecar" {
			t.Errorf("containers = %v, want [web sidecar]", info.Containers)
		}
	})

	t.Run("no containers", func(t *testing.T) {
		p := makeDiagPod("default", "no-ctr", func(p *corev1.Pod) {
			p.Spec.Containers = nil
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "no-ctr")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.Containers) != 0 {
			t.Errorf("expected no containers, got %v", info.Containers)
		}
	})

	t.Run("no IP yet", func(t *testing.T) {
		p := makeDiagPod("default", "pending-pod", func(p *corev1.Pod) {
			p.Status.PodIP = ""
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "pending-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.PodIP != "" {
			t.Errorf("expected empty PodIP, got %q", info.PodIP)
		}
	})

	t.Run("Calico annotation", func(t *testing.T) {
		p := makeDiagPod("default", "calico-pod", func(p *corev1.Pod) {
			p.Annotations = map[string]string{"cni.projectcalico.org/podIP": "10.244.1.5/32"}
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "calico-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(info.CNIAnnotation, "Calico:") {
			t.Errorf("CNIAnnotation = %q, want prefix 'Calico:'", info.CNIAnnotation)
		}
	})

	t.Run("Multus annotation", func(t *testing.T) {
		p := makeDiagPod("default", "multus-pod", func(p *corev1.Pod) {
			p.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": "macvlan-conf"}
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "multus-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(info.CNIAnnotation, "Multus:") {
			t.Errorf("CNIAnnotation = %q, want prefix 'Multus:'", info.CNIAnnotation)
		}
	})

	t.Run("kube-router annotation", func(t *testing.T) {
		p := makeDiagPod("default", "kr-pod", func(p *corev1.Pod) {
			p.Annotations = map[string]string{"kube-router.io/pod.name": "kr-pod"}
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "kr-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(info.CNIAnnotation, "Kube-Router:") {
			t.Errorf("CNIAnnotation = %q, want prefix 'Kube-Router:'", info.CNIAnnotation)
		}
	})

	t.Run("no labels — no policies", func(t *testing.T) {
		p := makeDiagPod("default", "bare-pod", func(p *corev1.Pod) {
			p.Labels = nil
		})
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "bare-pod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.NetworkPolicies) != 0 {
			t.Errorf("expected no policies, got %d", len(info.NetworkPolicies))
		}
	})

	t.Run("matching ingress policy", func(t *testing.T) {
		p := makeDiagPod("default", "web", func(p *corev1.Pod) {
			p.Labels = map[string]string{"app": "web"}
		})
		ingress := networkingv1.PolicyType("Ingress")
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "allow-web", Namespace: "default"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "web"},
				},
				PolicyTypes: []networkingv1.PolicyType{ingress},
			},
		}
		cs := fake.NewSimpleClientset(p, np)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "web")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.NetworkPolicies) != 1 {
			t.Fatalf("expected 1 policy, got %d", len(info.NetworkPolicies))
		}
		if info.NetworkPolicies[0].Name != "allow-web" {
			t.Errorf("policy name = %q, want 'allow-web'", info.NetworkPolicies[0].Name)
		}
		if info.NetworkPolicies[0].Direction != "ingress" {
			t.Errorf("direction = %q, want 'ingress'", info.NetworkPolicies[0].Direction)
		}
	})

	t.Run("matching ingress+egress policy", func(t *testing.T) {
		p := makeDiagPod("default", "api", func(p *corev1.Pod) {
			p.Labels = map[string]string{"app": "api"}
		})
		ingressType := networkingv1.PolicyType("Ingress")
		egressType := networkingv1.PolicyType("Egress")
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "api-policy", Namespace: "default"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "api"},
				},
				PolicyTypes: []networkingv1.PolicyType{ingressType, egressType},
			},
		}
		cs := fake.NewSimpleClientset(p, np)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.NetworkPolicies) != 1 {
			t.Fatalf("expected 1 policy, got %d", len(info.NetworkPolicies))
		}
		dir := info.NetworkPolicies[0].Direction
		if !strings.Contains(dir, "ingress") || !strings.Contains(dir, "egress") {
			t.Errorf("direction = %q, want ingress+egress combined", dir)
		}
	})

	t.Run("no matching policy", func(t *testing.T) {
		p := makeDiagPod("default", "worker", func(p *corev1.Pod) {
			p.Labels = map[string]string{"app": "worker"}
		})
		np := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "web-only", Namespace: "default"},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "web"},
				},
			},
		}
		cs := fake.NewSimpleClientset(p, np)
		c := newDiagClient(cs)
		info, err := c.GetPodNetworkInfo(ctx, "default", "worker")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.NetworkPolicies) != 0 {
			t.Errorf("expected 0 policies, got %d", len(info.NetworkPolicies))
		}
	})

	t.Run("missing pod returns error", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		c := newDiagClient(cs)
		_, err := c.GetPodNetworkInfo(ctx, "default", "ghost")
		if err == nil {
			t.Fatal("expected error for missing pod")
		}
	})
}

// ============================================================
// TestGetCNIStatus
// ============================================================

func TestGetCNIStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("Calico healthy", func(t *testing.T) {
		ds := makeDaemonSet("kube-system", "calico-node", 3, 3)
		cs := fake.NewSimpleClientset(ds)
		c := newDiagClient(cs)
		result, err := c.GetCNIStatus(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Plugin != "Calico" {
			t.Errorf("Plugin = %q, want Calico", result.Plugin)
		}
		if !result.Healthy {
			t.Error("expected Healthy=true for calico-node with desired==ready")
		}
		if len(result.DaemonSets) != 1 {
			t.Errorf("DaemonSets len = %d, want 1", len(result.DaemonSets))
		}
	})

	t.Run("Cilium degraded", func(t *testing.T) {
		ds := makeDaemonSet("kube-system", "cilium", 3, 1)
		cs := fake.NewSimpleClientset(ds)
		c := newDiagClient(cs)
		result, err := c.GetCNIStatus(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Plugin != "Cilium" {
			t.Errorf("Plugin = %q, want Cilium", result.Plugin)
		}
		// degraded: ready (1) != desired (3) so Healthy must be false
		if result.Healthy {
			t.Error("expected Healthy=false for degraded Cilium")
		}
	})

	t.Run("no CNI found", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		c := newDiagClient(cs)
		result, err := c.GetCNIStatus(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Plugin != "unknown" {
			t.Errorf("Plugin = %q, want 'unknown'", result.Plugin)
		}
		if result.Healthy {
			t.Error("expected Healthy=false when no CNI found")
		}
		if result.Error == "" {
			t.Error("expected non-empty Error when no CNI found")
		}
	})

	t.Run("multi-CNI one healthy one degraded => Healthy=false (post-fix semantic)", func(t *testing.T) {
		// Calico fully ready, Cilium degraded. Post-fix: ANY degraded -> false.
		calicoDS := makeDaemonSet("kube-system", "calico-node", 3, 3)
		ciliumDS := makeDaemonSet("kube-system", "cilium", 3, 1)
		cs := fake.NewSimpleClientset(calicoDS, ciliumDS)
		c := newDiagClient(cs)
		result, err := c.GetCNIStatus(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Must be false because cilium is degraded.
		if result.Healthy {
			t.Error("expected Healthy=false when ANY CNI DaemonSet is degraded")
		}
		if len(result.DaemonSets) != 2 {
			t.Errorf("DaemonSets len = %d, want 2", len(result.DaemonSets))
		}
	})

	t.Run("DS Get error is swallowed, others continue", func(t *testing.T) {
		// Only cilium exists; calico-node is absent — the fake returns 404 which is silently skipped.
		ds := makeDaemonSet("kube-system", "cilium", 2, 2)
		cs := fake.NewSimpleClientset(ds)
		c := newDiagClient(cs)
		result, err := c.GetCNIStatus(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should still discover Cilium.
		if result.Plugin != "Cilium" {
			t.Errorf("Plugin = %q, want Cilium", result.Plugin)
		}
	})
}

// ============================================================
// TestRunPodDNSCheck
// ============================================================

func TestRunPodDNSCheck(t *testing.T) {
	ctx := context.Background()

	// DNS check makes TWO exec calls: probe ("which tool?") then the
	// lookup. The `probe` field is the tool the container "reports"
	// having; `execOut` is what the lookup call returns. seqExec
	// threads them in order so each test row drives one full DNS run.
	tests := []struct {
		name        string
		probe       string // first-call output: tool name or "none"
		execOut     string // second-call output: actual lookup result
		execErr     error
		hostname    string
		wantResolved bool
		wantErrContains string
		wantMethod  string
	}{
		{
			name:         "happy resolve via getent",
			probe:        "getent",
			execOut:      "10.96.0.1      kubernetes.default.svc.cluster.local",
			hostname:     "kubernetes.default.svc.cluster.local",
			wantResolved: true,
			wantMethod:   "getent",
		},
		{
			name:         "happy resolve via nslookup",
			probe:        "nslookup",
			execOut:      "Server: 10.96.0.10\nName: myservice.default.svc.cluster.local\nAddress: 10.96.5.2",
			hostname:     "myservice.default.svc.cluster.local",
			wantResolved: true,
			wantMethod:   "nslookup",
		},
		{
			name:         "NXDOMAIN",
			probe:        "getent",
			execOut:      "NXDOMAIN: no such host",
			hostname:     "nonexistent.svc.cluster.local",
			wantResolved: false,
		},
		{
			name:         "SERVFAIL",
			probe:        "nslookup",
			execOut:      "SERVFAIL: server failure",
			hostname:     "broken.svc.cluster.local",
			wantResolved: false,
		},
		{
			name:            "empty lookup output",
			probe:           "getent",
			execOut:         "",
			hostname:        "empty.svc.cluster.local",
			wantResolved:    false,
		},
		{
			name:            "exec error bubbles into Error field",
			probe:           "getent",
			execErr:         errors.New("connection refused"),
			hostname:        "kubernetes.default.svc.cluster.local",
			wantResolved:    false,
			wantErrContains: "connection refused",
		},
		{
			name:            "no DNS tool available",
			probe:           "none",
			hostname:        "kubernetes.default.svc.cluster.local",
			wantResolved:    false,
			wantErrContains: "no DNS resolution tool",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewSimpleClientset()
			c := newDiagClient(cs)
			// Probe call returns the tool name; lookup call returns
			// the actual output (or the same execErr if set).
			c.runExec = seqExec(
				struct{ out string; err error }{tt.probe, tt.execErr},
				struct{ out string; err error }{tt.execOut, tt.execErr},
			)

			result, err := c.RunPodDNSCheck(ctx, "default", "my-pod", tt.hostname)
			if err != nil {
				t.Fatalf("RunPodDNSCheck returned Go error: %v (expected nil — errors should be surfaced via result.Error)", err)
			}
			if result.Resolved != tt.wantResolved {
				t.Errorf("Resolved = %v, want %v (probe=%q output=%q, err=%v)", result.Resolved, tt.wantResolved, tt.probe, tt.execOut, tt.execErr)
			}
			if tt.wantErrContains != "" && !strings.Contains(result.Error, tt.wantErrContains) {
				t.Errorf("Error = %q, want substring %q", result.Error, tt.wantErrContains)
			}
			if tt.wantMethod != "" && result.Method != tt.wantMethod {
				t.Errorf("Method = %q, want %q", result.Method, tt.wantMethod)
			}
		})
	}

	t.Run("default hostname used when empty", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		c := newDiagClient(cs)
		c.runExec = seqExec(
			struct{ out string; err error }{"getent", nil},
			struct{ out string; err error }{"10.96.0.1 kubernetes.default.svc.cluster.local", nil},
		)
		result, err := c.RunPodDNSCheck(ctx, "default", "my-pod", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Hostname != "kubernetes.default.svc.cluster.local" {
			t.Errorf("Hostname = %q, expected default", result.Hostname)
		}
	})
}

// ============================================================
// TestRunPodConnectivityCheck
// ============================================================

func TestRunPodConnectivityCheck(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		target         string
		port           int
		execOut        string
		execErr        error
		wantReachable  bool
		wantGoErr      bool
		wantErrContains string
	}{
		{
			name:          "reachable",
			target:        "10.96.0.1",
			port:          443,
			execOut:       "OK",
			wantReachable: true,
		},
		{
			name:            "connection refused",
			target:          "10.96.0.1",
			port:            9999,
			execOut:         "Connection refused",
			wantReachable:   false,
			wantErrContains: "Connection refused",
		},
		{
			name:            "timeout / UNREACHABLE",
			target:          "192.0.2.1",
			port:            80,
			execOut:         "UNREACHABLE",
			wantReachable:   false,
			wantErrContains: "UNREACHABLE",
		},
		{
			name:      "exec error bubbles into Error field",
			target:    "10.0.0.1",
			port:      8080,
			execErr:   errors.New("pod not found"),
			wantReachable: false,
			wantErrContains: "pod not found",
		},
		{
			// Invalid port — fixed surface is "set result.Error,
			// return nil error" so the frontend never has to
			// distinguish Go-level vs result-level failures.
			name:            "port 0 rejected",
			target:          "10.0.0.1",
			port:            0,
			wantReachable:   false,
			wantErrContains: "invalid port",
		},
		{
			name:            "port 70000 rejected",
			target:          "10.0.0.1",
			port:            70000,
			wantReachable:   false,
			wantErrContains: "invalid port",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewSimpleClientset()
			c := newDiagClient(cs)
			c.runExec = fakeExec(tt.execOut, tt.execErr)

			result, err := c.RunPodConnectivityCheck(ctx, "default", "my-pod", tt.target, tt.port)
			if tt.wantGoErr {
				if err == nil {
					t.Fatal("expected Go-level error for invalid port, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if result.Reachable != tt.wantReachable {
				t.Errorf("Reachable = %v, want %v", result.Reachable, tt.wantReachable)
			}
			if tt.wantErrContains != "" && !strings.Contains(result.Error, tt.wantErrContains) {
				t.Errorf("Error = %q, want substring %q", result.Error, tt.wantErrContains)
			}
		})
	}
}

// ============================================================
// TestRunPodNetworkDiagnostics
// ============================================================

func TestRunPodNetworkDiagnostics(t *testing.T) {
	ctx := context.Background()

	t.Run("full happy path", func(t *testing.T) {
		p := makeDiagPod("default", "diag-pod")
		calicoDS := makeDaemonSet("kube-system", "calico-node", 2, 2)
		cs := fake.NewSimpleClientset(p, calicoDS)
		c := newDiagClient(cs)
		c.runExec = fakeExec("10.96.0.1 kubernetes.default.svc.cluster.local", nil)

		summary, err := c.RunPodNetworkDiagnostics(ctx, "default", "diag-pod", "kubernetes.default.svc.cluster.local", 443)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.NetworkInfo == nil {
			t.Error("expected NetworkInfo to be populated")
		}
		if summary.DNSResult == nil {
			t.Error("expected DNSResult to be populated")
		}
		if summary.Connectivity == nil {
			t.Error("expected Connectivity to be populated")
		}
		if summary.CNIStatus == nil {
			t.Error("expected CNIStatus to be populated")
		}
	})

	t.Run("DNS fails but everything else succeeds", func(t *testing.T) {
		p := makeDiagPod("default", "diag-pod")
		calicoDS := makeDaemonSet("kube-system", "calico-node", 2, 2)
		cs := fake.NewSimpleClientset(p, calicoDS)
		c := newDiagClient(cs)
		// DNS exec returns an error; connectivity check gets a separate stub
		// since RunPodNetworkDiagnostics calls runExec internally for both.
		// We return empty string so DNS path sets error field, but partial result is set.
		c.runExec = fakeExec("", errors.New("dns resolution failed"))

		summary, err := c.RunPodNetworkDiagnostics(ctx, "default", "diag-pod", "kubernetes.default.svc.cluster.local", 443)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// NetworkInfo comes from API, not exec — should still succeed.
		if summary.NetworkInfo == nil {
			t.Error("expected NetworkInfo to be populated even when DNS fails")
		}
		// DNS result should be populated (with error field set inside).
		if summary.DNSResult == nil {
			t.Error("expected DNSResult to be set (with error inside) when exec fails")
		}
		// CNI status comes from API, should still populate.
		if summary.CNIStatus == nil {
			t.Error("expected CNIStatus to be populated even when DNS fails")
		}
	})

	t.Run("CNI failure does not break others", func(t *testing.T) {
		// No CNI daemonsets, but pod and exec work fine.
		p := makeDiagPod("default", "diag-pod")
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		c.runExec = fakeExec("10.96.0.1 kubernetes.default.svc.cluster.local", nil)

		summary, err := c.RunPodNetworkDiagnostics(ctx, "default", "diag-pod", "kubernetes.default.svc.cluster.local", 443)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.NetworkInfo == nil {
			t.Error("expected NetworkInfo even without CNI DaemonSets")
		}
		if summary.DNSResult == nil {
			t.Error("expected DNSResult even without CNI DaemonSets")
		}
		// CNI result should be populated but with plugin=unknown.
		if summary.CNIStatus == nil {
			t.Error("expected CNIStatus populated even when no CNI found")
		}
		if summary.CNIStatus != nil && summary.CNIStatus.Plugin != "unknown" {
			t.Errorf("CNIStatus.Plugin = %q, want 'unknown'", summary.CNIStatus.Plugin)
		}
	})

	t.Run("missing pod — NetworkInfo nil, rest proceeds", func(t *testing.T) {
		// No pod seeded — GetPodNetworkInfo will warn but not hard-fail RunPodNetworkDiagnostics.
		cs := fake.NewSimpleClientset()
		c := newDiagClient(cs)
		c.runExec = fakeExec("10.96.0.1 kubernetes.default.svc.cluster.local", nil)

		summary, err := c.RunPodNetworkDiagnostics(ctx, "default", "ghost", "kubernetes.default.svc.cluster.local", 443)
		if err != nil {
			t.Fatalf("unexpected top-level error: %v", err)
		}
		if summary.NetworkInfo != nil {
			t.Error("expected NetworkInfo=nil when pod does not exist")
		}
		// DNS and connectivity still attempted.
		if summary.DNSResult == nil {
			t.Error("expected DNSResult even when pod missing")
		}
	})

	t.Run("no targetHost — connectivity skipped", func(t *testing.T) {
		p := makeDiagPod("default", "diag-pod")
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		c.runExec = fakeExec("10.96.0.1 kubernetes.default.svc.cluster.local", nil)

		summary, err := c.RunPodNetworkDiagnostics(ctx, "default", "diag-pod", "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.Connectivity != nil {
			t.Error("expected Connectivity=nil when targetHost is empty")
		}
	})

	t.Run("zero port — connectivity skipped", func(t *testing.T) {
		p := makeDiagPod("default", "diag-pod")
		cs := fake.NewSimpleClientset(p)
		c := newDiagClient(cs)
		c.runExec = fakeExec("", nil)

		summary, err := c.RunPodNetworkDiagnostics(ctx, "default", "diag-pod", "some-host", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if summary.Connectivity != nil {
			t.Error("expected Connectivity=nil when port is 0")
		}
	})
}

// ============================================================
// TestGetCNIStatus_HealthySemanticPostFix
// ============================================================
// Dedicated table-driven tests asserting the post-fix Healthy semantics:
// Healthy is false if ANY discovered CNI DaemonSet is degraded.

func TestGetCNIStatus_HealthySemanticPostFix(t *testing.T) {
	ctx := context.Background()

	type dsSpec struct {
		name    string
		desired int32
		ready   int32
	}

	tests := []struct {
		name        string
		daemonSets  []dsSpec
		wantHealthy bool
	}{
		{"all ready 3/3", []dsSpec{{"calico-node", 3, 3}}, true},
		{"degraded 1/3", []dsSpec{{"calico-node", 3, 1}}, false},
		{"zero desired", []dsSpec{{"calico-node", 0, 0}}, false},
		{"two CNIs both healthy", []dsSpec{{"calico-node", 2, 2}, {"cilium", 2, 2}}, true},
		{"two CNIs one degraded", []dsSpec{{"calico-node", 2, 2}, {"cilium", 2, 1}}, false},
		{"two CNIs both degraded", []dsSpec{{"calico-node", 2, 0}, {"cilium", 2, 0}}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			for _, ds := range tt.daemonSets {
				objs = append(objs, makeDaemonSet("kube-system", ds.name, ds.desired, ds.ready))
			}
			cs := fake.NewSimpleClientset(objs...)
			c := newDiagClient(cs)
			result, err := c.GetCNIStatus(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Healthy != tt.wantHealthy {
				t.Errorf("Healthy = %v, want %v (daemonSets=%+v)", result.Healthy, tt.wantHealthy, tt.daemonSets)
			}
		})
	}
}

// ============================================================
// Ensure runExec field is usable (compile-time check embedded in tests above).
// The actual field and its type must be added to Client by the production-code
// fix. If the field is missing this file will not compile.
// ============================================================

// _ asserts the field exists on Client at package initialisation time.
var _ = func() bool {
	var c Client
	_ = fmt.Sprintf("%T", c.runExec) // non-nil field type check
	return true
}()
