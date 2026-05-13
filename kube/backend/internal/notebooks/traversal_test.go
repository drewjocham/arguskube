package notebooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStore_PathTraversal is the regression for the F1 finding from the
// post-PR security review: filepath.Clean alone does NOT strip leading
// `..` segments, so `filepath.Join(cacheDir, filepath.Clean("../etc"))`
// happily resolved to a sibling of the cache dir. With the resolveLocal
// helper in place, every Wails-RPC-callable method must reject any path
// that would escape st.cacheDir.
//
// The test exercises each public method (GetFile, SaveFile, DeleteFile,
// CreateFolder) against a battery of traversal patterns. We assert two
// things per case:
//  1. The call returns an error.
//  2. NO file or directory got created outside the cache root.
func TestStore_PathTraversal(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Drop a sentinel file alongside the cache dir. Each traversal
	// pattern below would (pre-fix) overwrite or delete this file. The
	// fix should make all of them error out, leaving the sentinel
	// untouched.
	parentDir := filepath.Dir(store.cacheDir)
	sentinel := filepath.Join(parentDir, "do-not-touch.txt")
	if err := os.WriteFile(sentinel, []byte("original"), 0600); err != nil {
		t.Fatalf("setup: write sentinel: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(sentinel) })

	travers := []string{
		"../do-not-touch.txt",
		"../../etc/passwd",
		"a/../../do-not-touch.txt",
		"foo/../../do-not-touch.txt",
		"/etc/passwd",
		"\x00../bad",
	}

	for _, p := range travers {
		p := p
		t.Run("Save_"+strings.ReplaceAll(p, "/", "_"), func(t *testing.T) {
			if err := store.SaveFile(ctx, p, "evil"); err == nil {
				t.Errorf("SaveFile(%q) should have errored", p)
			}
		})
		t.Run("Get_"+strings.ReplaceAll(p, "/", "_"), func(t *testing.T) {
			if _, err := store.GetFile(ctx, p); err == nil {
				t.Errorf("GetFile(%q) should have errored", p)
			}
		})
		t.Run("Delete_"+strings.ReplaceAll(p, "/", "_"), func(t *testing.T) {
			if err := store.DeleteFile(ctx, p); err == nil {
				t.Errorf("DeleteFile(%q) should have errored", p)
			}
		})
		t.Run("CreateFolder_"+strings.ReplaceAll(p, "/", "_"), func(t *testing.T) {
			if err := store.CreateFolder(ctx, p); err == nil {
				t.Errorf("CreateFolder(%q) should have errored", p)
			}
		})
	}

	// Sentinel must still hold its original content — no traversal
	// pattern was allowed to overwrite or delete it.
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("sentinel read after traversal attempts: %v (path was %s)", err, sentinel)
	}
	if string(got) != "original" {
		t.Fatalf("sentinel content changed — traversal succeeded; got %q", string(got))
	}
}

// TestStore_ValidPathsStillWork sanity-checks that the traversal guard
// doesn't accidentally reject legitimate paths.
func TestStore_ValidPathsStillWork(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	cases := []string{
		"notes.md",
		"folder/notes.md",
		"deep/nested/folder/notes.md",
		"unicode-名前.md",
	}
	for _, p := range cases {
		if err := store.SaveFile(ctx, p, "hello"); err != nil {
			t.Errorf("SaveFile(%q) errored unexpectedly: %v", p, err)
		}
	}
}
