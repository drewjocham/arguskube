package cd

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newFakeApplier(t *testing.T) *Applier {
	t.Helper()
	// dynamic/fake needs a runtime.Scheme; the client-go scheme covers
	// core/v1 + apps/v1, which is enough for the documents this test
	// suite hands ApplyManifest.
	dc := fake.NewSimpleDynamicClient(clientgoscheme.Scheme)

	// A trivial default REST mapper is sufficient — the empty-input
	// and decode-error paths never call into it.
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "", Version: "v1"},
	})

	return NewApplier(ApplierOptions{
		Logger:        discardLogger(),
		DynamicClient: dc,
		RESTMapper:    mapper,
	})
}

func TestNewApplierDefaultsFieldManager(t *testing.T) {
	t.Parallel()
	a := NewApplier(ApplierOptions{})
	if a.fieldManager != "arguscd" {
		t.Errorf("default fieldManager = %q, want arguscd", a.fieldManager)
	}
	if a.logger == nil {
		t.Error("logger should default to slog.Default(), not nil")
	}
}

func TestNewApplierRespectsExplicitFieldManager(t *testing.T) {
	t.Parallel()
	a := NewApplier(ApplierOptions{
		Logger:       discardLogger(),
		FieldManager: "my-controller",
	})
	if a.fieldManager != "my-controller" {
		t.Errorf("fieldManager = %q, want my-controller", a.fieldManager)
	}
}

func TestApplyManifestEmptyInputIsNoop(t *testing.T) {
	t.Parallel()
	a := newFakeApplier(t)
	if err := a.ApplyManifest(context.Background(), []byte{}); err != nil {
		t.Errorf("empty manifest should be a no-op, got %v", err)
	}
}

func TestApplyManifestWhitespaceInputIsNoop(t *testing.T) {
	t.Parallel()
	a := newFakeApplier(t)
	// Pure whitespace / blank-doc input decodes to nothing and must
	// not surface as an error.
	if err := a.ApplyManifest(context.Background(), []byte("\n\n---\n\n")); err != nil {
		t.Errorf("whitespace manifest should be a no-op, got %v", err)
	}
}

func TestApplyManifestRejectsInvalidYAML(t *testing.T) {
	t.Parallel()
	a := newFakeApplier(t)
	err := a.ApplyManifest(context.Background(), []byte(":\n\tnot: valid yaml: ["))
	if err == nil {
		t.Fatal("expected a decode error on malformed YAML")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "decode") &&
		!strings.Contains(strings.ToLower(err.Error()), "yaml") {
		t.Errorf("error should mention decode/yaml; got %v", err)
	}
}

func TestApplyManifestMissingRESTMappingErrors(t *testing.T) {
	t.Parallel()
	// Hand the applier a valid one-document manifest for an unknown
	// kind. The default mapper has no mapping for it, so ApplyManifest
	// must return the "failed to find REST mapping" error.
	a := newFakeApplier(t)
	manifest := []byte(`
apiVersion: nonexistent.example.com/v1
kind: NeverHeardOf
metadata:
  name: x
`)
	err := a.ApplyManifest(context.Background(), manifest)
	if err == nil {
		t.Fatal("expected REST-mapping error for unknown kind")
	}
	if !strings.Contains(err.Error(), "REST mapping") &&
		!strings.Contains(err.Error(), "mapping") {
		t.Errorf("error should mention REST mapping; got %v", err)
	}
}
