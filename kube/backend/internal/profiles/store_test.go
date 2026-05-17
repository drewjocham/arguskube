package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/argues/argus/internal/sqlitedb"
)

// newTestStore opens a fresh on-disk SQLite under t.TempDir so the
// migrations run and each test gets an isolated DB. In-memory DSNs
// don't compose well with sqlitedb.Open's path-construction, so we
// use a temp directory — cheap on Linux/macOS tmpfs.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("sqlitedb.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(db.DB, logger)
}

func TestNewStore(t *testing.T) {
	t.Run("nil logger does not panic", func(t *testing.T) {
		logger := slog.New(slog.DiscardHandler)
		db, err := sqlitedb.Open(t.TempDir(), logger)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		s := NewStore(db.DB, nil)
		if s == nil {
			t.Fatal("NewStore returned nil")
		}
		// Exercise a method to confirm the discard logger path works.
		if _, err := s.ListGroups(context.Background(), "u1"); err != nil {
			t.Fatalf("ListGroups: %v", err)
		}
	})
}

func TestListGroupsEmpty(t *testing.T) {
	s := newTestStore(t)
	got, err := s.ListGroups(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %d groups", len(got))
	}
	// Important: empty result must be a non-nil slice so JSON
	// encoding produces `[]` instead of `null`.
	if got == nil {
		t.Fatal("expected non-nil zero-length slice for clean JSON encoding")
	}
}

func TestSaveGroupValidation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		userID  string
		g       Group
		wantMsg string
	}{
		{"empty userID", "", Group{ID: "g1", Name: "n"}, "userID required"},
		{"empty groupID", "u1", Group{ID: "", Name: "n"}, "group ID required"},
		{"empty name", "u1", Group{ID: "g1", Name: ""}, "group name required"},
		{"whitespace-only name", "u1", Group{ID: "g1", Name: "   "}, "group name required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.SaveGroup(ctx, tc.userID, tc.g)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err=%v, want substring %q", err, tc.wantMsg)
			}
		})
	}
}

func TestSaveAndListGroups(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Save two groups for u1 and one for u2 — verify tenant isolation.
	g1, err := s.SaveGroup(ctx, "u1", Group{ID: "g-bravo", Name: "Bravo", Description: "second"})
	if err != nil {
		t.Fatalf("save g1: %v", err)
	}
	g2, err := s.SaveGroup(ctx, "u1", Group{ID: "g-alpha", Name: "Alpha"})
	if err != nil {
		t.Fatalf("save g2: %v", err)
	}
	if _, err := s.SaveGroup(ctx, "u2", Group{ID: "g-other", Name: "Other"}); err != nil {
		t.Fatalf("save u2: %v", err)
	}

	if g1.CreatedAt.IsZero() || g1.UpdatedAt.IsZero() {
		t.Errorf("save did not populate timestamps: %+v", g1)
	}

	got, err := s.ListGroups(ctx, "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 groups for u1, got %d", len(got))
	}
	// Sorted by name (case-insensitive). Alpha comes before Bravo.
	if got[0].ID != g2.ID || got[1].ID != g1.ID {
		t.Errorf("expected order [alpha, bravo], got [%s, %s]", got[0].Name, got[1].Name)
	}
	// u2's group must not leak in.
	for _, g := range got {
		if g.ID == "g-other" {
			t.Errorf("u2 group leaked into u1 listing")
		}
	}
}

func TestSaveGroupUpsertPreservesCreatedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "First"})
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	originalCreated := first.CreatedAt

	// Update — name changed, ID same. created_at must not move.
	second, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "Renamed", Description: "now described"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !second.CreatedAt.Equal(originalCreated) {
		t.Errorf("created_at moved on update: %v -> %v", originalCreated, second.CreatedAt)
	}
	if second.Name != "Renamed" || second.Description != "now described" {
		t.Errorf("update did not apply: %+v", second)
	}
}

func TestSaveAndListVariants(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}

	snap := json.RawMessage(`{"appearance":{"theme":"dark"}}`)
	v, err := s.SaveVariant(ctx, "u1", "g1", Variant{
		ID:       "v1",
		Name:     "Workspace",
		Version:  "1.2",
		Snapshot: snap,
	})
	if err != nil {
		t.Fatalf("save variant: %v", err)
	}
	if v.ParentID != "g1" {
		t.Errorf("ParentID not set, got %q", v.ParentID)
	}
	if string(v.Snapshot) != string(snap) {
		t.Errorf("snapshot round-trip mismatch: got %s want %s", v.Snapshot, snap)
	}

	got, err := s.ListGroups(ctx, "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || len(got[0].Variants) != 1 {
		t.Fatalf("expected 1 group with 1 variant, got %d groups", len(got))
	}
	if got[0].Variants[0].ID != "v1" {
		t.Errorf("variant lookup wrong: %+v", got[0].Variants[0])
	}
}

func TestSaveVariantDefaults(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}

	v, err := s.SaveVariant(ctx, "u1", "g1", Variant{ID: "v1", Name: "default"})
	if err != nil {
		t.Fatalf("save variant: %v", err)
	}
	if v.Version != "1.0" {
		t.Errorf("expected default version 1.0, got %q", v.Version)
	}
	if string(v.Snapshot) != "{}" {
		t.Errorf("expected default snapshot {}, got %q", string(v.Snapshot))
	}
}

func TestSaveVariantRejectsCrossUserGroup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "owned-by-u1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}
	// u2 must not be able to drop a variant into u1's group, even by
	// guessing the group ID. This is the multi-tenant boundary check.
	_, err := s.SaveVariant(ctx, "u2", "g1", Variant{ID: "v1", Name: "intrusion"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-user save, got %v", err)
	}
}

func TestDeleteGroupCascadesVariants(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}
	for _, vid := range []string{"v1", "v2", "v3"} {
		if _, err := s.SaveVariant(ctx, "u1", "g1", Variant{ID: vid, Name: vid}); err != nil {
			t.Fatalf("save variant %s: %v", vid, err)
		}
	}

	if err := s.DeleteGroup(ctx, "u1", "g1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	got, err := s.ListGroups(ctx, "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 groups after delete, got %d", len(got))
	}

	// Variants table should no longer have the cascaded rows.
	for _, vid := range []string{"v1", "v2", "v3"} {
		err := s.db.QueryRowContext(ctx,
			`SELECT id FROM profile_variants WHERE id = ?`, vid,
		).Scan(new(string))
		if err == nil {
			t.Errorf("variant %s survived group delete", vid)
		}
	}
}

func TestDeleteGroupRejectsCrossUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "owned-by-u1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}
	err := s.DeleteGroup(ctx, "u2", "g1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// u1's group must still be there.
	got, _ := s.ListGroups(ctx, "u1")
	if len(got) != 1 {
		t.Errorf("u1's group was deleted by u2's call")
	}
}

func TestDeleteGroupClearsActivePointer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}
	if _, err := s.SaveVariant(ctx, "u1", "g1", Variant{ID: "v1", Name: "v"}); err != nil {
		t.Fatalf("save variant: %v", err)
	}
	if err := s.SetActive(ctx, "u1", "g1", "v1"); err != nil {
		t.Fatalf("set active: %v", err)
	}
	if err := s.DeleteGroup(ctx, "u1", "g1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	active, err := s.GetActive(ctx, "u1")
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.GroupID != "" || active.VariantID != "" {
		t.Errorf("active pointer survived group delete: %+v", active)
	}
}

func TestDeleteVariant(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}
	if _, err := s.SaveVariant(ctx, "u1", "g1", Variant{ID: "v1", Name: "v1"}); err != nil {
		t.Fatalf("save variant: %v", err)
	}

	if err := s.DeleteVariant(ctx, "u1", "g1", "v1"); err != nil {
		t.Fatalf("delete variant: %v", err)
	}

	got, _ := s.ListGroups(ctx, "u1")
	if len(got[0].Variants) != 0 {
		t.Errorf("variant survived delete")
	}
}

func TestDeleteVariantRejectsCrossUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}
	if _, err := s.SaveVariant(ctx, "u1", "g1", Variant{ID: "v1", Name: "v1"}); err != nil {
		t.Fatalf("save variant: %v", err)
	}

	err := s.DeleteVariant(ctx, "u2", "g1", "v1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-user delete, got %v", err)
	}
}

func TestSetGetActive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Empty by default.
	a, err := s.GetActive(ctx, "u1")
	if err != nil {
		t.Fatalf("get active (empty): %v", err)
	}
	if a.GroupID != "" || a.VariantID != "" {
		t.Errorf("expected zero-value Active, got %+v", a)
	}

	if err := s.SetActive(ctx, "u1", "g1", "v1"); err != nil {
		t.Fatalf("set active: %v", err)
	}
	a, _ = s.GetActive(ctx, "u1")
	if a.GroupID != "g1" || a.VariantID != "v1" {
		t.Errorf("set/get round-trip failed: %+v", a)
	}

	// Updating overwrites.
	if err := s.SetActive(ctx, "u1", "g2", "v2"); err != nil {
		t.Fatalf("re-set: %v", err)
	}
	a, _ = s.GetActive(ctx, "u1")
	if a.GroupID != "g2" || a.VariantID != "v2" {
		t.Errorf("update did not overwrite: %+v", a)
	}
}

func TestSetActiveTenantIsolation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.SetActive(ctx, "u1", "g1", "v1"); err != nil {
		t.Fatalf("set u1: %v", err)
	}
	if err := s.SetActive(ctx, "u2", "g2", "v2"); err != nil {
		t.Fatalf("set u2: %v", err)
	}
	a1, _ := s.GetActive(ctx, "u1")
	a2, _ := s.GetActive(ctx, "u2")
	if a1.GroupID != "g1" || a2.GroupID != "g2" {
		t.Errorf("tenant isolation broken: u1=%+v u2=%+v", a1, a2)
	}
}

func TestSnapshotJSONRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.SaveGroup(ctx, "u1", Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("save group: %v", err)
	}

	// Realistic snapshot shape from the frontend's ProfileSnapshot type.
	snap := json.RawMessage(`{
		"appearance": {"theme": "dark", "fontSize": 14},
		"navVisibility": {"visible": {"clusters": true, "alerts": false}},
		"sectionTabs": {"tabs": {"home": "overview"}},
		"uiPrefs": {"rightPanelWidth": 380},
		"savedFilters": [
			{"id": "f1", "name": "errors-last-1h", "query": "status:error", "filters": [], "limit": null,
			 "createdAt": "2026-05-17T00:00:00Z", "updatedAt": "2026-05-17T00:00:00Z"}
		]
	}`)

	saved, err := s.SaveVariant(ctx, "u1", "g1", Variant{
		ID: "v1", Name: "Daily", Version: "1.0", Snapshot: snap,
	})
	if err != nil {
		t.Fatalf("save variant: %v", err)
	}

	// Round-trip parse to confirm SQLite hasn't mangled escape chars.
	var roundTripped map[string]any
	if err := json.Unmarshal(saved.Snapshot, &roundTripped); err != nil {
		t.Fatalf("read-back snapshot is not valid JSON: %v\n  raw=%s", err, saved.Snapshot)
	}
	if app, ok := roundTripped["appearance"].(map[string]any); !ok || app["theme"] != "dark" {
		t.Errorf("snapshot lost data on round-trip: %+v", roundTripped)
	}
}
