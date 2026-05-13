package secretstore

import (
	"context"
	"sync"
	"testing"
)

// MemoryStore is the test-mode default; if its contract drifts the
// macOS impl can't be a drop-in replacement.

func TestMemoryStore_RoundTrip(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if v, ok, err := s.Get(ctx, "missing"); err != nil || ok || v != "" {
		t.Fatalf("missing key should be empty/false/nil, got %q %v %v", v, ok, err)
	}

	if err := s.Set(ctx, "argus.session.token", "abc123"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, ok, err := s.Get(ctx, "argus.session.token")
	if err != nil || !ok || v != "abc123" {
		t.Fatalf("get after set, want abc123/true/nil, got %q %v %v", v, ok, err)
	}
}

func TestMemoryStore_DeleteIsIdempotent(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	// Delete on a key that was never set must not error.
	if err := s.Delete(ctx, "never-set"); err != nil {
		t.Fatalf("delete on missing key returned error: %v", err)
	}
	_ = s.Set(ctx, "k", "v")
	if err := s.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := s.Get(ctx, "k"); ok {
		t.Fatalf("key should be gone after delete")
	}
}

func TestMemoryStore_OverwriteReplaces(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	_ = s.Set(ctx, "k", "v1")
	_ = s.Set(ctx, "k", "v2")
	v, ok, _ := s.Get(ctx, "k")
	if !ok || v != "v2" {
		t.Fatalf("want v2/true, got %q/%v", v, ok)
	}
}

func TestMemoryStore_ConcurrentReadsAndWrites(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = s.Set(ctx, "k", "v")
		}(i)
		go func() {
			defer wg.Done()
			_, _, _ = s.Get(ctx, "k")
		}()
	}
	wg.Wait()
}

func TestMemoryStore_BackendLabel(t *testing.T) {
	if NewMemoryStore().Backend() != "in-memory only" {
		t.Errorf("backend label drift")
	}
}

func TestNew_AlwaysReturnsAStore(t *testing.T) {
	// Non-mutating: we explicitly do NOT write through this Store in
	// the unit test, because on macOS New() picks a Keychain backend
	// which would create entries in the developer's real keychain.
	// Integration tests against the Keychain belong behind a build tag.
	s := New("svc")
	if s == nil {
		t.Fatalf("New() must always return a non-nil Store")
	}
	if s.Backend() == "" {
		t.Errorf("Store should report a non-empty backend label")
	}
}
