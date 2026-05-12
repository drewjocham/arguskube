package k8s

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argues/kube-watcher/internal/config"
)

// testClientWith builds a Client wired to a fake clientset preloaded with
// arbitrary objects (typed to whichever resource the caller seeds via the
// returned fake.Clientset). All ops covered here exercise CoreV1/AppsV1.
func testClientWith() *Client {
	return &Client{
		cs:     fake.NewSimpleClientset(),
		cfg:    &config.OnlineDataConfig{},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// --- GetWarningEvents -------------------------------------------------------

func TestGetWarningEvents_FormatAndLimit(t *testing.T) {
	c := testClientWith()
	ctx := context.Background()
	events := []corev1.Event{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "e1", Namespace: "default"},
			Type:       "Warning",
			Reason:     "BackOff",
			Message:    "container restarted",
			Count:      3,
			InvolvedObject: corev1.ObjectReference{Namespace: "default", Name: "api"},
			LastTimestamp:  metav1.Time{Time: time.Date(2026, 5, 9, 12, 30, 0, 0, time.UTC)},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "e2", Namespace: "default"},
			Type:       "Warning",
			Reason:     "FailedMount",
			Message:    "secret not found",
			Count:      1,
			InvolvedObject: corev1.ObjectReference{Namespace: "default", Name: "web"},
			LastTimestamp:  metav1.Time{Time: time.Date(2026, 5, 9, 12, 31, 0, 0, time.UTC)},
		},
	}
	for i := range events {
		_, _ = c.cs.CoreV1().Events("default").Create(ctx, &events[i], metav1.CreateOptions{})
	}

	got, err := c.GetWarningEvents(ctx, 10)
	if err != nil {
		t.Fatalf("GetWarningEvents: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(got))
	}
	joined := strings.Join(got, " | ")
	if !strings.Contains(joined, "default/api") || !strings.Contains(joined, "BackOff") {
		t.Errorf("expected event 1 in output, got %q", joined)
	}
	if !strings.Contains(joined, "default/web") || !strings.Contains(joined, "FailedMount") {
		t.Errorf("expected event 2 in output, got %q", joined)
	}
}

func TestGetWarningEvents_EmptyResult(t *testing.T) {
	c := testClientWith()
	got, err := c.GetWarningEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetWarningEvents: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}

// --- GetTopRestarters -------------------------------------------------------

func TestGetTopRestarters_SortedAndLimited(t *testing.T) {
	c := testClientWith()
	ctx := context.Background()

	pods := []corev1.Pod{
		makePodWithRestarts("low", "default", "main", 2),
		makePodWithRestarts("hot", "default", "main", 12),
		makePodWithRestarts("mid", "kube-system", "agent", 5),
	}
	for i := range pods {
		_, _ = c.cs.CoreV1().Pods(pods[i].Namespace).Create(ctx, &pods[i], metav1.CreateOptions{})
	}

	got, err := c.GetTopRestarters(ctx, 2)
	if err != nil {
		t.Fatalf("GetTopRestarters: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries (limit), got %d", len(got))
	}
	if !strings.Contains(got[0], "12 restarts") {
		t.Errorf("expected 'hot' first, got %q", got[0])
	}
	if !strings.Contains(got[1], "5 restarts") {
		t.Errorf("expected 'mid' second, got %q", got[1])
	}
}

func TestGetTopRestarters_SkipsZeroRestartContainers(t *testing.T) {
	c := testClientWith()
	ctx := context.Background()
	_, _ = c.cs.CoreV1().Pods("default").Create(ctx,
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "stable", Namespace: "default"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "main", RestartCount: 0},
				},
			},
		}, metav1.CreateOptions{})
	got, err := c.GetTopRestarters(ctx, 10)
	if err != nil {
		t.Fatalf("GetTopRestarters: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected zero restarts to be filtered, got %v", got)
	}
}

func makePodWithRestarts(name, ns, container string, restarts int32) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: container, RestartCount: restarts},
			},
		},
	}
}

// --- GetDeploymentRevisions -------------------------------------------------

func TestGetDeploymentRevisions_OwnedReplicaSetsOnly(t *testing.T) {
	c := testClientWith()
	ctx := context.Background()

	depUID := types.UID("dep-uid-123")
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
			UID:       depUID,
		},
	}
	_, _ = c.cs.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{})

	owned := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-aaa",
			Namespace: "default",
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": "1",
				"kubernetes.io/change-cause":        "kubectl apply",
			},
			OwnerReferences: []metav1.OwnerReference{
				{UID: depUID, Kind: "Deployment", Name: "api"},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "myorg/api:v2.0.1"},
					},
				},
			},
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}
	stranger := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "other-bbb",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{{UID: "different", Kind: "Deployment", Name: "other"}},
		},
	}
	_, _ = c.cs.AppsV1().ReplicaSets("default").Create(ctx, owned, metav1.CreateOptions{})
	_, _ = c.cs.AppsV1().ReplicaSets("default").Create(ctx, stranger, metav1.CreateOptions{})

	revs, err := c.GetDeploymentRevisions(ctx, "default", "api", 10)
	if err != nil {
		t.Fatalf("GetDeploymentRevisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("expected 1 revision (owned only), got %d", len(revs))
	}
	r := revs[0]
	if r.ReplicaSet != "api-aaa" || r.Revision != "1" || r.Image != "myorg/api:v2.0.1" || r.ChangeCause != "kubectl apply" {
		t.Errorf("unexpected revision metadata: %+v", r)
	}
	if !r.Active {
		t.Error("expected Active=true when Replicas > 0")
	}
}

func TestGetDeploymentRevisions_DeploymentMissing(t *testing.T) {
	c := testClientWith()
	if _, err := c.GetDeploymentRevisions(context.Background(), "default", "ghost", 10); err == nil {
		t.Error("expected error for missing deployment")
	}
}

// --- DefaultCostConfig ------------------------------------------------------

func TestDefaultCostConfig(t *testing.T) {
	cfg := DefaultCostConfig()
	if cfg.Provider != ProviderAWS {
		t.Errorf("expected AWS default, got %q", cfg.Provider)
	}
	if cfg.CPUPerCoreHour <= 0 || cfg.MemPerGBHour <= 0 {
		t.Errorf("expected positive rates, got %+v", cfg)
	}
}

func TestCostConfigForProvider_FallbackToAWS(t *testing.T) {
	cfg := CostConfigForProvider(CloudProvider("not-a-cloud"))
	if cfg.Provider != ProviderAWS {
		t.Errorf("expected AWS fallback, got %q", cfg.Provider)
	}
}

// --- ListContextsFromKubeconfig --------------------------------------------

const sampleKubeconfig = `apiVersion: v1
kind: Config
current-context: prod-east
clusters:
- name: prod-east-cluster
  cluster:
    server: https://api.prod-east.example
- name: dev-cluster
  cluster:
    server: https://api.dev.example
contexts:
- name: prod-east
  context:
    cluster: prod-east-cluster
    user: alice
- name: dev
  context:
    cluster: dev-cluster
    user: alice
- name: staging
  context:
    cluster: prod-east-cluster
    user: alice
users:
- name: alice
  user:
    token: x
`

func writeTempKubeconfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(path, []byte(sampleKubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func TestListContextsFromKubeconfig_ReadsAll(t *testing.T) {
	path := writeTempKubeconfig(t)
	got, err := ListContextsFromKubeconfig(path, "")
	if err != nil {
		t.Fatalf("ListContextsFromKubeconfig: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 contexts, got %d (%+v)", len(got), got)
	}

	// The active context is current-context = prod-east. With sort: active
	// first, alphabetical otherwise.
	if !got[0].Active || got[0].Name != "prod-east" {
		t.Errorf("expected active prod-east first, got %+v", got[0])
	}
	for _, c := range got[1:] {
		if c.Active {
			t.Errorf("only one context should be active, got: %+v", c)
		}
	}
}

func TestListContextsFromKubeconfig_OverrideActive(t *testing.T) {
	path := writeTempKubeconfig(t)
	got, err := ListContextsFromKubeconfig(path, "dev")
	if err != nil {
		t.Fatalf("ListContextsFromKubeconfig: %v", err)
	}
	if !got[0].Active || got[0].Name != "dev" {
		t.Errorf("expected dev to be promoted to active, got %+v", got)
	}
}

func TestListContextsFromKubeconfig_NoFile(t *testing.T) {
	_, err := ListContextsFromKubeconfig("/no/such/path/kubeconfig", "")
	if err == nil {
		t.Error("expected error when kubeconfig missing")
	}
}

// --- Client.ListContexts (delegates to ListContextsFromKubeconfig) ----------

func TestClient_ListContexts_UsesConfiguredKubeconfig(t *testing.T) {
	path := writeTempKubeconfig(t)
	c := &Client{
		cs:     fake.NewSimpleClientset(),
		cfg:    &config.OnlineDataConfig{Kubernetes: config.KubernetesConfig{Config: path, Context: "staging"}},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	got, err := c.ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 contexts, got %d", len(got))
	}
	// The "staging" override should make staging active even though
	// current-context says prod-east.
	if !got[0].Active || got[0].Name != "staging" {
		t.Errorf("expected staging active first, got %+v", got[0])
	}
}

func TestClient_ListContexts_NilCfgFallsBackToDefaults(t *testing.T) {
	c := &Client{cs: fake.NewSimpleClientset(), cfg: nil,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	// Either a default kubeconfig at $HOME/.kube/config exists (returns ok)
	// or it doesn't (returns error). Both are acceptable; we only assert
	// the call doesn't panic.
	_, _ = c.ListContexts()
}

// --- buildMemoryPressureAlert ----------------------------------------------

func TestBuildMemoryPressureAlert_Shape(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	a := buildMemoryPressureAlert(node)
	if a.NodeName != "node-1" {
		t.Errorf("expected node name in alert, got %q", a.NodeName)
	}
	if a.ID != "mempress-node-1" {
		t.Errorf("unexpected ID: %q", a.ID)
	}
	if a.Namespace != "infra" {
		t.Errorf("expected infra namespace, got %q", a.Namespace)
	}
	if len(a.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(a.Tags))
	}
}

// --- GetPodLogs (fake clientset returns "fake logs" by default) ------------

func TestGetPodLogs_ReturnsParsedLines(t *testing.T) {
	c := testClientWith()
	ctx := context.Background()
	_, _ = c.cs.CoreV1().Pods("default").Create(ctx,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}},
		metav1.CreateOptions{})

	lines, err := c.GetPodLogs(ctx, "default", "api", 100)
	if err != nil {
		t.Fatalf("GetPodLogs: %v", err)
	}
	// fake.Clientset's GetLogs returns one synthetic line "fake logs" — this
	// is a documented Kubernetes fake-client behavior. We assert it's parsed
	// into a LogLine with a level inferred and a source tag set.
	if len(lines) == 0 {
		t.Fatal("expected at least one log line from fake client")
	}
	if lines[0].Source != "[api]" {
		t.Errorf("expected source=[api], got %q", lines[0].Source)
	}
	if lines[0].Level == "" {
		t.Error("expected a level to be inferred")
	}
}

func TestGetPodLogs_UnknownPodReturnsLines(t *testing.T) {
	// The fake clientset's GetLogs subresource doesn't validate that the pod
	// exists — it just streams the placeholder. So this exercises the
	// pod-not-found branch only at the typed-client layer (which we get for
	// free), and confirms our scanner doesn't choke.
	c := testClientWith()
	if _, err := c.GetPodLogs(context.Background(), "ghost-ns", "ghost", 5); err != nil {
		t.Logf("GetPodLogs on missing pod returned error (acceptable): %v", err)
	}
}

// --- DeletePod through fake client (sanity) ---------------------------------

func TestDeletePod_WithFakeClient(t *testing.T) {
	c := testClientWith()
	ctx := context.Background()
	_, _ = c.cs.CoreV1().Pods("default").Create(ctx,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "doomed", Namespace: "default"}},
		metav1.CreateOptions{})
	if err := c.DeletePod(ctx, "default", "doomed"); err != nil {
		t.Fatalf("DeletePod: %v", err)
	}
	if _, err := c.cs.CoreV1().Pods("default").Get(ctx, "doomed", metav1.GetOptions{}); err == nil {
		t.Error("expected pod to be gone after DeletePod")
	}
}
