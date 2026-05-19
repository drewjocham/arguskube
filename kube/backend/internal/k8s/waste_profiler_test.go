package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
