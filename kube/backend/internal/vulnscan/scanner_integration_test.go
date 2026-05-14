package vulnscan

// White-box integration tests that wire up a fake Kubernetes client and a
// stub runTrivy function so that ScanAll, ScanSingleImage, enumerateImages,
// and scanImage can be exercised without a real cluster or trivy binary.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// newTestLogger returns a silent logger suitable for tests.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// trivyJSON is a minimal valid Trivy JSON report with one CRITICAL finding.
const trivyJSONOneCritical = `{
	"Results": [{
		"Target": "test-image:v1",
		"Vulnerabilities": [{
			"VulnerabilityID": "CVE-2024-0001",
			"PkgName": "curl",
			"Severity": "CRITICAL",
			"Title": "Heap overflow",
			"FixedVersion": "8.4.0"
		}]
	}]
}`

// trivyJSONClean is a Trivy JSON report with no vulnerabilities.
const trivyJSONClean = `{"Results": []}`

// makeFakeScanner creates a Scanner with a fake kube client containing a
// single running pod that references the given image in the given namespace.
func makeFakeScanner(image, namespace string, runner trivyRunner) *Scanner {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: image,
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	cs := fake.NewSimpleClientset(pod)
	s := New(cs, newTestLogger())
	s.runTrivy = runner
	return s
}

// ---------------------------------------------------------------------------
// enumerateImages
// ---------------------------------------------------------------------------

func TestEnumerateImages_ReturnsPodImages(t *testing.T) {
	s := makeFakeScanner("nginx:1.25", "default", nil)

	images, err := s.enumerateImages(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].name != "nginx:1.25" {
		t.Errorf("image name = %q, want nginx:1.25", images[0].name)
	}
	if images[0].namespace != "default" {
		t.Errorf("namespace = %q, want default", images[0].namespace)
	}
}

func TestEnumerateImages_DeduplicatesImages(t *testing.T) {
	// Two pods with the same image.
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "shared:v1"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "prod"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "shared:v1"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := fake.NewSimpleClientset(pod1, pod2)
	s := New(cs, newTestLogger())

	images, err := s.enumerateImages(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 deduplicated image, got %d", len(images))
	}
}

func TestEnumerateImages_IncludesInitContainers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "app", Image: "app:v1"}},
			InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1.36"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := fake.NewSimpleClientset(pod)
	s := New(cs, newTestLogger())

	images, err := s.enumerateImages(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images (app + busybox), got %d", len(images))
	}
}

func TestEnumerateImages_NoClusterConnection(t *testing.T) {
	s := New(nil, newTestLogger())
	_, err := s.enumerateImages(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when cs is nil")
	}
}

// ---------------------------------------------------------------------------
// scanImage (via injected runTrivy)
// ---------------------------------------------------------------------------

func TestScanImage_HappyPath(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(trivyJSONOneCritical), nil
	}

	result, err := s.scanImage(context.Background(), clusterImage{name: "test-image:v1", namespace: "default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Critical != 1 {
		t.Errorf("Critical = %d, want 1", result.Critical)
	}
	if result.Status != "Vulnerable" {
		t.Errorf("Status = %q, want Vulnerable", result.Status)
	}
	if result.Name != "test-image:v1" {
		t.Errorf("Name = %q, want test-image:v1", result.Name)
	}
}

func TestScanImage_RunnerError(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return nil, fmt.Errorf("trivy not found")
	}

	_, err := s.scanImage(context.Background(), clusterImage{name: "img", namespace: "ns"})
	if err == nil {
		t.Fatal("expected error from failing runner, got nil")
	}
}

func TestScanImage_MalformedJSON(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(`{bad json`), nil
	}

	_, err := s.scanImage(context.Background(), clusterImage{name: "img", namespace: "ns"})
	if err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
}

// ---------------------------------------------------------------------------
// ScanAll
// ---------------------------------------------------------------------------

func TestScanAll_HappyPath(t *testing.T) {
	s := makeFakeScanner("app:v1", "default", func(_ context.Context, _ string) ([]byte, error) {
		return []byte(trivyJSONOneCritical), nil
	})

	results, err := s.ScanAll(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Critical != 1 {
		t.Errorf("Critical = %d, want 1", results[0].Critical)
	}
	if results[0].ID != "img-1" {
		t.Errorf("ID = %q, want img-1", results[0].ID)
	}
}

func TestScanAll_UpdatesCache(t *testing.T) {
	s := makeFakeScanner("app:v1", "default", func(_ context.Context, _ string) ([]byte, error) {
		return []byte(trivyJSONClean), nil
	})

	// Before ScanAll, List returns demo data (non-empty).
	_, _ = s.ScanAll(context.Background(), "")

	// After ScanAll, List must return the scanned data, not demo data.
	listed := s.List()
	if len(listed) != 1 {
		t.Errorf("List() after ScanAll = %d items, want 1", len(listed))
	}
	if listed[0].Name != "app:v1" {
		t.Errorf("List()[0].Name = %q, want app:v1", listed[0].Name)
	}
}

func TestScanAll_ScanFailurePlaceholder(t *testing.T) {
	s := makeFakeScanner("broken:v1", "default", func(_ context.Context, _ string) ([]byte, error) {
		return nil, fmt.Errorf("simulated trivy failure")
	})

	results, err := s.ScanAll(context.Background(), "")
	if err != nil {
		t.Fatalf("ScanAll itself should not error on per-image failure: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 placeholder result, got %d", len(results))
	}
	if results[0].Status != "Error" {
		t.Errorf("Status = %q, want Error", results[0].Status)
	}
}

func TestScanAll_NoCluster(t *testing.T) {
	s := New(nil, newTestLogger())
	_, err := s.ScanAll(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when cluster client is nil")
	}
}

func TestScanAll_SortsResultsByScore(t *testing.T) {
	// Two pods: one clean, one with a critical.
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-clean", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "clean:v1"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-vuln", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "vuln:v1"}}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := fake.NewSimpleClientset(pod1, pod2)
	s := New(cs, newTestLogger())
	s.runTrivy = func(_ context.Context, image string) ([]byte, error) {
		if image == "vuln:v1" {
			return []byte(trivyJSONOneCritical), nil
		}
		return []byte(trivyJSONClean), nil
	}

	results, err := s.ScanAll(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Most critical should be first.
	if results[0].Critical == 0 {
		t.Errorf("first result should be the vulnerable image (critical>0), got status %q", results[0].Status)
	}
}

// ---------------------------------------------------------------------------
// ScanSingleImage
// ---------------------------------------------------------------------------

func TestScanSingleImage_HappyPath(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(trivyJSONOneCritical), nil
	}

	msg, err := s.ScanSingleImage(context.Background(), "test-image:v1", "trivy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == "" {
		t.Error("expected non-empty result message")
	}
	if !containsStr(msg, "1 critical") {
		t.Errorf("message %q should mention '1 critical'", msg)
	}
}

func TestScanSingleImage_UpdatesExistingCacheEntry(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(trivyJSONClean), nil
	}

	// Prime the cache with a manual entry.
	s.mu.Lock()
	s.results = []ScannedImage{{ID: "img-1", Name: "known:v1", Namespace: "default"}}
	s.mu.Unlock()

	_, err := s.ScanSingleImage(context.Background(), "known:v1", "trivy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the existing entry was updated in place (still only 1 entry).
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.results) != 1 {
		t.Errorf("cache should still have 1 entry after update, got %d", len(s.results))
	}
	if s.results[0].ID != "img-1" {
		t.Errorf("existing ID should be preserved, got %q", s.results[0].ID)
	}
}

func TestScanSingleImage_AppendsNewImageToCache(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return []byte(trivyJSONClean), nil
	}

	// Cache is empty.
	_, err := s.ScanSingleImage(context.Background(), "brand-new:v1", "trivy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.results) != 1 {
		t.Errorf("expected 1 entry in cache, got %d", len(s.results))
	}
	if s.results[0].Name != "brand-new:v1" {
		t.Errorf("cache entry Name = %q, want brand-new:v1", s.results[0].Name)
	}
}

func TestScanSingleImage_RunnerError(t *testing.T) {
	s := New(nil, newTestLogger())
	s.runTrivy = func(_ context.Context, _ string) ([]byte, error) {
		return nil, fmt.Errorf("binary not found")
	}

	_, err := s.ScanSingleImage(context.Background(), "img:v1", "trivy")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// List — with populated cache
// ---------------------------------------------------------------------------

func TestList_WithCache(t *testing.T) {
	s := New(nil, newTestLogger())
	s.mu.Lock()
	s.results = []ScannedImage{{ID: "img-1", Name: "cached:v1"}}
	s.mu.Unlock()

	results := s.List()
	if len(results) != 1 {
		t.Fatalf("expected 1 result from cache, got %d", len(results))
	}
	if results[0].Name != "cached:v1" {
		t.Errorf("Name = %q, want cached:v1", results[0].Name)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
