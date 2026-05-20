package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// "Sometime errors for same namespaces" turned out to be the API
// returning a transient error on one call and succeeding on the next.
// These tests pin the retry+classify behavior so a regression there
// reproduces the bug we just fixed.

func TestIsTransientK8sError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		err       error
		transient bool
	}{
		{"nil", nil, false},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"forbidden", apierrors.NewForbidden(schema.GroupResource{Resource: "deployments"}, "x", errors.New("nope")), false},
		{"not found", apierrors.NewNotFound(schema.GroupResource{Resource: "deployments"}, "x"), false},
		{"unauthorized", apierrors.NewUnauthorized("nope"), false},
		{"too many requests", apierrors.NewTooManyRequests("slow down", 1), true},
		{"server timeout", apierrors.NewServerTimeout(schema.GroupResource{Resource: "deployments"}, "list", 1), true},
		{"service unavailable", apierrors.NewServiceUnavailable("upstream down"), true},
		{"internal error", apierrors.NewInternalError(errors.New("boom")), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isTransientK8sError(tc.err)
			if got != tc.transient {
				t.Fatalf("transient=%v, want %v", got, tc.transient)
			}
		})
	}
}

func TestListDeploymentsWithRetry_RecoversOnTransient(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	calls := 0
	cs.PrependReactor("list", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		calls++
		if calls < 3 {
			return true, nil, apierrors.NewServerTimeout(schema.GroupResource{Resource: "deployments"}, "list", 1)
		}
		return true, &appsv1.DeploymentList{ListMeta: metav1.ListMeta{}}, nil
	})

	c := &Client{cs: cs}
	got, err := c.listDeploymentsWithRetry(context.Background(), "default")
	if err != nil {
		t.Fatalf("expected eventual success after retries, got %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result on success")
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts (2 transient + 1 success), got %d", calls)
	}
}

func TestListDeploymentsWithRetry_ShortCircuitsOnPermanent(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	calls := 0
	cs.PrependReactor("list", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		calls++
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "deployments"}, "x", errors.New("rbac says no"))
	})

	c := &Client{cs: cs}
	_, err := c.listDeploymentsWithRetry(context.Background(), "kube-system")
	if err == nil {
		t.Fatal("expected forbidden to propagate")
	}
	if !apierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %T %v", err, err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 attempt on permanent error, got %d", calls)
	}
}

func TestListDeploymentsWithRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	calls := 0
	cs.PrependReactor("list", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		calls++
		return true, nil, apierrors.NewServerTimeout(schema.GroupResource{Resource: "deployments"}, "list", 1)
	})

	c := &Client{cs: cs}
	_, err := c.listDeploymentsWithRetry(context.Background(), "default")
	if err == nil {
		t.Fatal("expected eventual failure after 3 transient attempts")
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts then give up, got %d", calls)
	}
}

func TestFriendlyWasteError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		err      error
		contains string
	}{
		{"forbidden",
			apierrors.NewForbidden(schema.GroupResource{Resource: "deployments"}, "x", errors.New("rbac")),
			"permission"},
		{"unauthorized",
			apierrors.NewUnauthorized("expired"),
			"sign in"},
		{"not found",
			apierrors.NewNotFound(schema.GroupResource{Resource: "deployments"}, "x"),
			"not found"},
		{"timeout",
			apierrors.NewTimeoutError("upstream slow", 1),
			"timed out"},
		{"too many requests",
			apierrors.NewTooManyRequests("slow down", 1),
			"rate-limited"},
		{"deadline exceeded",
			context.DeadlineExceeded,
			"took too long"},
		{"generic",
			errors.New("random unrelated thing"),
			"could not list deployments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := friendlyWasteError("ns-foo", tc.err)
			if out == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(out.Error(), tc.contains) {
				t.Fatalf("expected error to contain %q, got %q", tc.contains, out.Error())
			}
		})
	}
}

// ProfileClusterWaste fans out across namespaces — verify it
// aggregates per-namespace profiles, sorts by score, and records
// per-namespace errors inline instead of aborting the whole call.
func TestProfileClusterWaste_AggregatesAndSorts(t *testing.T) {
	t.Parallel()

	// Two namespaces. "high-waste" gets a deployment with a fat CPU
	// request that pushes its score up; "low-waste" stays clean.
	const fatCPU = int64(3000) // 3 cores in millis — well above the "critical" floor (2000)
	highDep := makeWasteDep("high-waste", "fat", fatCPU)
	lowDep := makeWasteDep("low-waste", "lean", 50) // tiny request → low score

	cs := fake.NewSimpleClientset(
		makeNs("high-waste"),
		makeNs("low-waste"),
		highDep,
		lowDep,
	)
	c := &Client{cs: cs}

	got, err := c.ProfileClusterWaste(context.Background())
	if err != nil {
		t.Fatalf("ProfileClusterWaste: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(got))
	}
	// "critical" sorts before "low" so high-waste must come first.
	if got[0].Namespace != "high-waste" {
		t.Errorf("first ns = %q, want %q (sort by score, then name)", got[0].Namespace, "high-waste")
	}
	if got[1].Namespace != "low-waste" {
		t.Errorf("second ns = %q, want %q", got[1].Namespace, "low-waste")
	}
	if got[0].Score != "critical" {
		t.Errorf("high-waste score = %q, want critical", got[0].Score)
	}
}

func TestProfileClusterWaste_RecordsPerNamespaceErrors(t *testing.T) {
	t.Parallel()

	cs := fake.NewSimpleClientset(makeNs("forbidden-ns"), makeNs("ok-ns"))
	// Make the deployments list for forbidden-ns fail with a 403.
	cs.PrependReactor("list", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		la := action.(clienttesting.ListAction)
		if la.GetNamespace() == "forbidden-ns" {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "deployments"}, "x", errors.New("rbac"))
		}
		return false, nil, nil
	})
	c := &Client{cs: cs}

	got, err := c.ProfileClusterWaste(context.Background())
	if err != nil {
		t.Fatalf("ProfileClusterWaste should not fail when one ns errors: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected both namespaces in result, got %d", len(got))
	}
	var forbidden *WasteProfile
	for i := range got {
		if got[i].Namespace == "forbidden-ns" {
			forbidden = &got[i]
		}
	}
	if forbidden == nil {
		t.Fatal("forbidden-ns missing from result")
	}
	if forbidden.Error == "" {
		t.Errorf("forbidden-ns should have inline Error, got empty")
	}
	if forbidden.Score != "unknown" {
		t.Errorf("forbidden-ns score = %q, want unknown", forbidden.Score)
	}
}

func TestWasteScoreOrder(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"critical": 0,
		"high":     1,
		"medium":   2,
		"low":      3,
		"unknown":  4,
		"":        4,
		"garbage":  4,
	}
	for in, want := range cases {
		if got := wasteScoreOrder(in); got != want {
			t.Errorf("wasteScoreOrder(%q) = %d, want %d", in, got, want)
		}
	}
}

// makeNs + makeWasteDep are tiny constructors used by the cluster
// tests above. Kept local rather than wired into the shared
// detailClient helper because the waste profiler doesn't need the
// same config/logger plumbing.

func makeNs(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func makeWasteDep(ns, name string, cpuMillis int64) *appsv1.Deployment {
	q := resourceMillis(cpuMillis)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: q,
								},
							},
						},
					},
				},
			},
		},
	}
}

func resourceMillis(m int64) resource.Quantity {
	return *resource.NewMilliQuantity(m, resource.DecimalSI)
}
