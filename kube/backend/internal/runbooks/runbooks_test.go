package runbooks

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	appconfig "github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/notebooks"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := New(nil, logger)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	// Override dir to our temp.
	store.dir = tmpDir
	return store
}

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	store, err := New(nil, logger)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if store == nil {
		t.Fatal("New() returned nil")
	}
	if store.dir == "" {
		t.Error("expected non-empty dir")
	}
	if !strings.Contains(store.dir, "runbooks") {
		t.Errorf("dir should contain 'runbooks', got %q", store.dir)
	}
}

func TestCreate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rb, err := store.Create(ctx, "Pod Crash Recovery", "CrashLoopBackOff")
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if rb.Name != "Pod Crash Recovery" {
		t.Errorf("rb.Name = %q, want %q", rb.Name, "Pod Crash Recovery")
	}
	if rb.Trigger != "CrashLoopBackOff" {
		t.Errorf("rb.Trigger = %q, want %q", rb.Trigger, "CrashLoopBackOff")
	}
	if rb.Status != "draft" {
		t.Errorf("rb.Status = %q, want %q", rb.Status, "draft")
	}
	if rb.ID == "" {
		t.Error("rb.ID should not be empty")
	}
	if rb.Steps < 1 {
		t.Errorf("expected at least 1 step, got %d", rb.Steps)
	}
}

func TestCreateDuplicate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "My Runbook", "test")
	if err != nil {
		t.Fatalf("first Create() returned error: %v", err)
	}

	_, err = store.Create(ctx, "My Runbook", "test")
	if err == nil {
		t.Fatal("expected error for duplicate runbook")
	}
}

func TestSaveAndGet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Save a runbook.
	content := `---
name: Memory Cleanup
trigger: high-memory
status: ready
---
# Memory Cleanup

1. Check memory usage
2. Identify top consumers
3. Restart as needed
`
	id := "memory-cleanup"
	if err := store.Save(ctx, id, content); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Get it back.
	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if got != content {
		t.Errorf("Get() content mismatch\ngot:  %q\nwant: %q", got, content)
	}
}

func TestGetNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent runbook")
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create and then delete.
	rb, err := store.Create(ctx, "Temp Runbook", "test")
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if err := store.Delete(ctx, rb.ID); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	// Verify it's gone.
	_, err = store.Get(ctx, rb.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.Delete(ctx, "does-not-exist")
	if err != nil {
		t.Errorf("deleting nonexistent runbook should succeed, got: %v", err)
	}
}

func TestList(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a few runbooks.
	names := []string{"Alpha Runbook", "Beta Runbook", "Gamma Runbook"}
	for _, name := range names {
		_, err := store.Create(ctx, name, "trigger")
		if err != nil {
			t.Fatalf("Create(%q) returned error: %v", name, err)
		}
	}

	runbooks, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	if len(runbooks) != 3 {
		t.Fatalf("expected 3 runbooks, got %d", len(runbooks))
	}

	// Verify sort order.
	for i := 1; i < len(runbooks); i++ {
		if runbooks[i].Name < runbooks[i-1].Name {
			t.Errorf("runbooks not sorted: %q before %q", runbooks[i-1].Name, runbooks[i].Name)
		}
	}
}

func TestListEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	runbooks, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(runbooks) != 0 {
		t.Errorf("expected 0 runbooks, got %d", len(runbooks))
	}
}

func TestNameToID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Pod Crash Recovery", "pod-crash-recovery"},
		{"Node Maintenance", "node-maintenance"},
		{"  spaces   ", "--spaces---"},
		{"UPPERCASE", "uppercase"},
		{"special!@#chars", "specialchars"},
		{"a-b_c", "a-b_c"},
		{"123abc", "123abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := nameToID(tt.input)
			if got != tt.want {
				t.Errorf("nameToID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRunbook(t *testing.T) {
	content := `---
name: My Runbook
trigger: pod-crash
status: ready
lastRun: 2024-01-15
---
# My Runbook

1. Check pod logs
2. Describe pod
3. Restart pod
`
	rb := parseRunbook("my-runbook.md", content, time.Now())
	if rb.Name != "My Runbook" {
		t.Errorf("Name = %q, want %q", rb.Name, "My Runbook")
	}
	if rb.Trigger != "pod-crash" {
		t.Errorf("Trigger = %q, want %q", rb.Trigger, "pod-crash")
	}
	if rb.Status != "ready" {
		t.Errorf("Status = %q, want %q", rb.Status, "ready")
	}
	if rb.LastRun != "2024-01-15" {
		t.Errorf("LastRun = %q, want %q", rb.LastRun, "2024-01-15")
	}
	if rb.Steps != 3 {
		t.Errorf("Steps = %d, want 3", rb.Steps)
	}
	if rb.ID != "my-runbook" {
		t.Errorf("ID = %q, want %q", rb.ID, "my-runbook")
	}
}

func TestParseRunbookNoFrontmatter(t *testing.T) {
	content := `# Quick Runbook

1. Do the thing
2. Verify it worked
`
	rb := parseRunbook("quick.md", content, time.Now())
	if rb.Name != "quick" {
		t.Errorf("Name = %q, want %q", rb.Name, "quick")
	}
	if rb.Steps != 2 {
		t.Errorf("Steps = %d, want 2", rb.Steps)
	}
	if rb.Status != "draft" {
		t.Errorf("Status = %q, want %q", rb.Status, "draft")
	}
}

func TestParseRunbookEmpty(t *testing.T) {
	rb := parseRunbook("empty.md", "", time.Now())
	if rb.Name != "empty" {
		t.Errorf("Name = %q, want %q", rb.Name, "empty")
	}
	if rb.Steps == 0 {
		t.Errorf("expected at least 1 default step, got 0")
	}
}

func TestRunbookWithNotebookStore(t *testing.T) {
	// Create a store with an unconfigured notebooks Store (no S3).
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	tmpDir := t.TempDir()

	nbCfg := &appconfig.OnlineDataConfig{
		S3: appconfig.S3Config{Bucket: ""},
	}
	nbStore, err := notebooks.New(nbCfg, logger)
	if err != nil {
		t.Fatalf("notebooks.New() returned error: %v", err)
	}

	store, err := New(nbStore, logger)
	if err != nil {
		t.Fatalf("New() with notebook store returned error: %v", err)
	}
	store.dir = tmpDir

	ctx := context.Background()
	rb, err := store.Create(ctx, "Synced Runbook", "alert")
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if rb.Name != "Synced Runbook" {
		t.Errorf("Name = %q, want %q", rb.Name, "Synced Runbook")
	}
}

func TestSaveWithMDExtensionStrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	content := "# Strip test"
	err := store.Save(ctx, "test-runbook.md", content)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Should be retrievable both with and without .md.
	got, err := store.Get(ctx, "test-runbook")
	if err != nil {
		t.Fatalf("Get() without .md returned error: %v", err)
	}
	if got != content {
		t.Errorf("Get() content mismatch")
	}

	got2, err := store.Get(ctx, "test-runbook.md")
	if err != nil {
		t.Fatalf("Get() with .md returned error: %v", err)
	}
	if got2 != content {
		t.Errorf("Get() with .md content mismatch")
	}
}

func TestIDToPath(t *testing.T) {
	store := newTestStore(t)

	p1 := store.idToPath("my-runbook")
	if !strings.HasSuffix(p1, "my-runbook.md") {
		t.Errorf("expected path ending in 'my-runbook.md', got %q", p1)
	}

	p2 := store.idToPath("my-runbook.md")
	if !strings.HasSuffix(p2, "my-runbook.md") {
		t.Errorf("expected path ending in 'my-runbook.md', got %q", p2)
	}
}

func TestLifecycleFull(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create
	rb, err := store.Create(ctx, "Full Lifecycle Test", "lifecycle")
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	// List includes it
	runbooks, _ := store.List(ctx)
	found := false
	for _, r := range runbooks {
		if r.ID == rb.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("runbook not found in list after creation")
	}

	// Delete
	if err := store.Delete(ctx, rb.ID); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	// List excludes it
	runbooks, _ = store.List(ctx)
	for _, r := range runbooks {
		if r.ID == rb.ID {
			t.Error("runbook still found in list after deletion")
			break
		}
	}
}
