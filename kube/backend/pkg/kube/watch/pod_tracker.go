package watch

import (
	"context"
	"log/slog"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
)

// PodTracker maintains a live in-memory index of which pods exist so that
// alert enrichment can cheaply check pod existence without API calls.
type PodTracker struct {
	cs     kubernetes.Interface
	logger *slog.Logger

	mu   sync.RWMutex
	pods map[string]podEntry // key = "namespace/name"

	cancel context.CancelFunc
}

type podEntry struct {
	UID string
}

// NewPodTracker creates a PodTracker.
func NewPodTracker(cs kubernetes.Interface, logger *slog.Logger) *PodTracker {
	return &PodTracker{
		cs:     cs,
		logger: logger,
		pods:   make(map[string]podEntry),
	}
}

// Start begins watching pods. It is non-blocking; watching runs in a goroutine.
func (pt *PodTracker) Start(ctx context.Context) error {
	ctx, pt.cancel = context.WithCancel(ctx)

	// Seed with current pods.
	list, err := pt.cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	pt.mu.Lock()
	for i := range list.Items {
		p := &list.Items[i]
		pt.pods[p.Namespace+"/"+p.Name] = podEntry{UID: string(p.UID)}
	}
	pt.mu.Unlock()

	go pt.watch(ctx, list.ResourceVersion)
	return nil
}

// Stop cancels the background watcher.
func (pt *PodTracker) Stop() {
	if pt.cancel != nil {
		pt.cancel()
	}
}

// PodExists returns true if the pod is currently known.
func (pt *PodTracker) PodExists(namespace, name string) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	_, ok := pt.pods[namespace+"/"+name]
	return ok
}

// GetPodUID returns the UID of a pod and whether it was found.
func (pt *PodTracker) GetPodUID(namespace, name string) (string, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	e, ok := pt.pods[namespace+"/"+name]
	return e.UID, ok
}

func (pt *PodTracker) watch(ctx context.Context, rv string) {
	for {
		if ctx.Err() != nil {
			return
		}
		watcher, err := pt.cs.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{ResourceVersion: rv})
		if err != nil {
			pt.logger.Warn("pod tracker: watch error, will retry", slog.String("error", err.Error()))
			return
		}
		for ev := range watcher.ResultChan() {
			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			key := pod.Namespace + "/" + pod.Name
			switch ev.Type {
			case watch.Added, watch.Modified:
				pt.mu.Lock()
				pt.pods[key] = podEntry{UID: string(pod.UID)}
				pt.mu.Unlock()
			case watch.Deleted:
				pt.mu.Lock()
				delete(pt.pods, key)
				pt.mu.Unlock()
			}
			rv = pod.ResourceVersion
		}
	}
}
