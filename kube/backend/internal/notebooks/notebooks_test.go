package notebooks

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/argues/argus/internal/config"
)

// newTestStore creates a Store with no S3 configured (local-only mode), using a
// temp cache directory so tests don't pollute the real cache.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := &appconfig.OnlineDataConfig{
		S3: appconfig.S3Config{
			Bucket: "",
		},
	}

	store, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	// Override cache dir to our temp.
	store.cacheDir = tmpDir
	return store
}

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Run("local-only mode when no bucket configured", func(t *testing.T) {
		cfg := &appconfig.OnlineDataConfig{
			S3: appconfig.S3Config{Bucket: ""},
		}
		store, err := New(cfg, logger)
		if err != nil {
			t.Fatalf("New() returned error: %v", err)
		}
		if store == nil {
			t.Fatal("New() returned nil")
		}
		if store.IsConfigured() {
			t.Error("expected store to be unconfigured (local only)")
		}
	})

	t.Run("local-only mode when credentials missing", func(t *testing.T) {
		cfg := &appconfig.OnlineDataConfig{
			S3: appconfig.S3Config{
				Bucket: "my-bucket",
			},
		}
		store, err := New(cfg, logger)
		if err != nil {
			t.Fatalf("New() returned error: %v", err)
		}
		if store == nil {
			t.Fatal("New() returned nil")
		}
		if store.IsConfigured() {
			t.Error("expected store to be unconfigured when credentials missing")
		}
	})
}

func TestSaveAndGetFile(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	content := "# Test Notebook\n\nThis is a test notebook."
	path := "test-notebook.md"

	err := store.SaveFile(ctx, path, content)
	if err != nil {
		t.Fatalf("SaveFile() returned error: %v", err)
	}

	got, err := store.GetFile(ctx, path)
	if err != nil {
		t.Fatalf("GetFile() returned error: %v", err)
	}
	if got != content {
		t.Errorf("GetFile() = %q, want %q", got, content)
	}
}

func TestSaveFileAddsMDExtension(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	content := "# Auto-extended"
	path := "auto-extend" // no .md

	err := store.SaveFile(ctx, path, content)
	if err != nil {
		t.Fatalf("SaveFile() returned error: %v", err)
	}

	// Should be saved with .md extension.
	got, err := store.GetFile(ctx, "auto-extend.md")
	if err != nil {
		t.Fatalf("GetFile() with .md returned error: %v", err)
	}
	if got != content {
		t.Errorf("GetFile() = %q, want %q", got, content)
	}
}

func TestGetFileNotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetFile(ctx, "nonexistent.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDeleteFile(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create a file first.
	content := "# To be deleted"
	path := "delete-me.md"
	if err := store.SaveFile(ctx, path, content); err != nil {
		t.Fatalf("SaveFile() returned error: %v", err)
	}

	// Delete it.
	if err := store.DeleteFile(ctx, path); err != nil {
		t.Fatalf("DeleteFile() returned error: %v", err)
	}

	// Verify it's gone.
	_, err := store.GetFile(ctx, path)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestDeleteFileNonexistent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteFile(ctx, "does-not-exist.md")
	if err != nil {
		t.Errorf("deleting nonexistent file should succeed, got: %v", err)
	}
}

func TestListFiles(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	files := map[string]string{
		"notes/quickstart.md": "# Quickstart",
		"notes/advanced.md":   "# Advanced",
		"root-note.md":        "# Root",
		"archive/old.md":      "# Old",
	}

	for path, content := range files {
		if err := store.SaveFile(ctx, path, content); err != nil {
			t.Fatalf("SaveFile(%q) returned error: %v", path, err)
		}
	}

	entries, err := store.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles() returned error: %v", err)
	}

	// Should have 2 root entries: notes folder and root-note.md (archive is inside notes... no, archive and notes are separate).
	// Actually: notes/ is a folder, archive/ is a folder, root-note.md is a root-level file.
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 root entries, got %d: %+v", len(entries), entries)
	}

	var rootFileFound, notesFolderFound, archiveFolderFound bool
	for _, e := range entries {
		switch e.Path {
		case "root-note.md":
			rootFileFound = true
			if e.Type != "file" {
				t.Errorf("root-note.md type = %q, want %q", e.Type, "file")
			}
		case "notes":
			notesFolderFound = true
			if e.Type != "folder" {
				t.Errorf("notes type = %q, want %q", e.Type, "folder")
			}
			if len(e.Children) != 2 {
				t.Errorf("expected 2 children in notes, got %d", len(e.Children))
			}
		case "archive":
			archiveFolderFound = true
		}
	}

	if !rootFileFound {
		t.Error("root-note.md not found in listing")
	}
	if !notesFolderFound {
		t.Error("notes folder not found in listing")
	}
	if !archiveFolderFound {
		t.Error("archive folder not found in listing")
	}
}

func TestListFilesEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entries, err := store.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles() returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries in empty store, got %d", len(entries))
	}
}

func TestCreateFolder(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateFolder(ctx, "my-folder")
	if err != nil {
		t.Fatalf("CreateFolder() returned error: %v", err)
	}

	// Verify folder exists in cache dir.
	info, err := os.Stat(filepath.Join(store.cacheDir, "my-folder"))
	if err != nil {
		t.Fatalf("folder was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created entry is not a directory")
	}
}

func TestCreateFolderAddsTrailingSlash(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateFolder(ctx, "no-slash")
	if err != nil {
		t.Fatalf("CreateFolder() returned error: %v", err)
	}

	// Should have been created as "no-slash" directory.
	info, err := os.Stat(filepath.Join(store.cacheDir, "no-slash"))
	if err != nil {
		t.Fatalf("folder was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created entry is not a directory")
	}
}

func TestGetCacheDir(t *testing.T) {
	store := newTestStore(t)
	dir := store.GetCacheDir()
	if dir == "" {
		t.Error("GetCacheDir() returned empty string")
	}
	// The cache dir should exist on disk.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("GetCacheDir() = %q, directory does not exist", dir)
	}
}

func TestIsConfigured(t *testing.T) {
	store := newTestStore(t)
	if store.IsConfigured() {
		t.Error("local-only store should not be configured")
	}
}

func TestBuildTree(t *testing.T) {
	files := map[string]*FileEntry{
		"root.md": {ID: "root", Name: "root.md", Path: "root.md", Type: "file"},
		"sub/file.md": {ID: "sub/file", Name: "file.md", Path: "sub/file.md", Type: "file"},
	}
	folders := map[string]*FileEntry{
		"sub": {ID: "sub", Name: "sub", Path: "sub", Type: "folder", Children: []FileEntry{}},
	}

	roots := buildTree(files, folders)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}

	// sub folder should have 1 child.
	for _, r := range roots {
		if r.Type == "folder" && r.Name == "sub" {
			if len(r.Children) != 1 {
				t.Errorf("expected 1 child in 'sub' folder, got %d", len(r.Children))
			}
			if r.Children[0].Name != "file.md" {
				t.Errorf("expected child named 'file.md', got %q", r.Children[0].Name)
			}
		}
	}
}
