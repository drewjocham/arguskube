package loadtest

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Scaler is the engine's Kubernetes dependency. The interface is kept
// minimal so tests can drop in a fake without standing up the full
// fake.Clientset (though kubeclientScaler does use that under the
// hood for production).
type Scaler interface {
	// Scale sets the deployment's replicas to the target value.
	Scale(ctx context.Context, namespace, deployment string, replicas int32) error
	// WaitForReplicas blocks until status.readyReplicas == target,
	// or ctx expires. Returns ctx.Err() on timeout.
	WaitForReplicas(ctx context.Context, namespace, deployment string, target int32) error
	// Observe returns the current (spec, ready) replica counts.
	// Used to populate the ScaleEvent stream during the run.
	Observe(ctx context.Context, namespace, deployment string) (spec int32, ready int32, err error)
}

// kubeScaler is the production Scaler backed by client-go.
type kubeScaler struct {
	cs       kubernetes.Interface
	pollEvery time.Duration
}

// NewKubeScaler builds a Scaler from a client-go Interface. pollEvery
// is how often WaitForReplicas re-checks; defaults to 500ms.
func NewKubeScaler(cs kubernetes.Interface, pollEvery time.Duration) Scaler {
	if pollEvery <= 0 {
		pollEvery = 500 * time.Millisecond
	}
	return &kubeScaler{cs: cs, pollEvery: pollEvery}
}

func (k *kubeScaler) Scale(ctx context.Context, ns, name string, replicas int32) error {
	dep, err := k.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get deployment %s/%s: %w", ns, name, err)
	}
	dep.Spec.Replicas = &replicas
	_, err = k.cs.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update deployment %s/%s replicas=%d: %w", ns, name, replicas, err)
	}
	return nil
}

func (k *kubeScaler) WaitForReplicas(ctx context.Context, ns, name string, target int32) error {
	t := time.NewTicker(k.pollEvery)
	defer t.Stop()
	for {
		dep, err := k.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			if dep.Status.ReadyReplicas == target {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (k *kubeScaler) Observe(ctx context.Context, ns, name string) (int32, int32, error) {
	dep, err := k.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, 0, err
	}
	var spec int32
	if dep.Spec.Replicas != nil {
		spec = *dep.Spec.Replicas
	}
	return spec, dep.Status.ReadyReplicas, nil
}

// Compile-check that kubeScaler implements Scaler.
var _ Scaler = (*kubeScaler)(nil)

// (unused — appsv1 imported only via the client-go path above; keep
// the package referenced so future helpers that build a Deployment
// directly compile cleanly.)
var _ = appsv1.SchemeGroupVersion
