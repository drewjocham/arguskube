package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/argues/argus/internal/auth"
	"github.com/argues/argus/internal/config"
	profilespkg "github.com/argues/argus/internal/profiles"
	"github.com/argues/argus/internal/sqlitedb"
)

// app_profiles_test.go — integration coverage for the Wails-bound
// CRUD on *App. Constructs a real App (DB, auth store, profile
// store) and exercises the methods the same way the frontend does
// over Wails / HTTP. The store-layer unit tests live alongside the
// store; these tests focus on the bindings: arg passing, auth gate,
// nil-store handling, multi-tenant scoping at the App boundary.

// devAuthBypass token — every method takes a session token but
// SetupAuth(DevMode: true) resolves any token (including "") to the
// devModeUserID synthetic user. Tests use "" to make the call site
// read like the frontend's "anonymous local dev" mode.
const devAuthBypass = ""

// newProfilesTestApp builds an App with a real SQLite DB + real
// profile store + devMode-bypass auth. Cleanup closes the DB.
func newProfilesTestApp(t *testing.T) *App {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("sqlitedb.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	a := &App{
		ctx:    context.Background(),
		logger: logger,
		db:     db.DB,
	}
	a.profiles = profilespkg.NewStore(db.DB, logger)
	a.SetupAuth(auth.NewStore(db.DB, logger), config.AuthConfig{DevMode: true})
	return a
}

func TestListProfileGroupsEmpty(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	got, err := a.ListProfileGroups(devAuthBypass)
	if err != nil {
		t.Fatalf("ListProfileGroups: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil zero-length slice for clean JSON; got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 groups on fresh DB, got %d", len(got))
	}
}

func TestProfileGroupCRUDRoundTrip(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	// Create
	saved, err := a.SaveProfileGroup(devAuthBypass, profilespkg.Group{
		ID: "g-uuid-1", Name: "Daily", Description: "morning rotation",
	})
	if err != nil {
		t.Fatalf("SaveProfileGroup: %v", err)
	}
	if saved.ID != "g-uuid-1" || saved.Name != "Daily" {
		t.Errorf("unexpected saved group: %+v", saved)
	}
	if saved.CreatedAt.IsZero() {
		t.Errorf("created_at not populated")
	}

	// List sees it.
	listed, err := a.ListProfileGroups(devAuthBypass)
	if err != nil {
		t.Fatalf("list after save: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "g-uuid-1" {
		t.Fatalf("list didn't surface the save: %+v", listed)
	}

	// Update (upsert with same ID, changed fields).
	updated, err := a.SaveProfileGroup(devAuthBypass, profilespkg.Group{
		ID: "g-uuid-1", Name: "Renamed", Description: "now described",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Renamed" || updated.Description != "now described" {
		t.Errorf("update didn't apply: %+v", updated)
	}
	// created_at must survive an upsert — frontends rely on it for
	// "this is the older profile" UI hints.
	if !updated.CreatedAt.Equal(saved.CreatedAt) {
		t.Errorf("created_at moved on update: %v -> %v", saved.CreatedAt, updated.CreatedAt)
	}

	// Delete
	if err := a.DeleteProfileGroup(devAuthBypass, "g-uuid-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	listed, _ = a.ListProfileGroups(devAuthBypass)
	if len(listed) != 0 {
		t.Errorf("group survived delete: %+v", listed)
	}
}

func TestProfileVariantCRUDRoundTrip(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	if _, err := a.SaveProfileGroup(devAuthBypass, profilespkg.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	snap := json.RawMessage(`{"appearance":{"theme":"dark","fontSize":16},"navVisibility":{"visible":{"alerts":true}}}`)
	saved, err := a.SaveProfileVariant(devAuthBypass, "g1", profilespkg.Variant{
		ID: "v1", Name: "Desk", Version: "1.2", Snapshot: snap,
	})
	if err != nil {
		t.Fatalf("SaveProfileVariant: %v", err)
	}
	if saved.ParentID != "g1" || saved.Name != "Desk" {
		t.Errorf("unexpected saved variant: %+v", saved)
	}
	if string(saved.Snapshot) != string(snap) {
		t.Errorf("snapshot mangled by round-trip:\n  in:  %s\n  out: %s", snap, saved.Snapshot)
	}

	// List nests variants under their group.
	listed, _ := a.ListProfileGroups(devAuthBypass)
	if len(listed) != 1 || len(listed[0].Variants) != 1 || listed[0].Variants[0].ID != "v1" {
		t.Fatalf("variant not nested in group list: %+v", listed)
	}

	if err := a.DeleteProfileVariant(devAuthBypass, "g1", "v1"); err != nil {
		t.Fatalf("DeleteProfileVariant: %v", err)
	}
	listed, _ = a.ListProfileGroups(devAuthBypass)
	if len(listed[0].Variants) != 0 {
		t.Errorf("variant survived delete: %+v", listed)
	}
}

func TestActiveProfileRoundTrip(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	// Default: zero-value Active, no error.
	active, err := a.GetActiveProfile(devAuthBypass)
	if err != nil {
		t.Fatalf("GetActiveProfile (initial): %v", err)
	}
	if active.GroupID != "" || active.VariantID != "" {
		t.Errorf("expected empty initial active, got %+v", active)
	}

	if err := a.SetActiveProfile(devAuthBypass, "g1", "v1"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}
	active, _ = a.GetActiveProfile(devAuthBypass)
	if active.GroupID != "g1" || active.VariantID != "v1" {
		t.Errorf("set/get round-trip failed: %+v", active)
	}

	// Clearing via the documented (empty, empty) convention.
	if err := a.SetActiveProfile(devAuthBypass, "", ""); err != nil {
		t.Fatalf("SetActiveProfile clear: %v", err)
	}
	active, _ = a.GetActiveProfile(devAuthBypass)
	if active.GroupID != "" || active.VariantID != "" {
		t.Errorf("clear did not reset active: %+v", active)
	}
}

func TestDeleteGroupCascadesViaWailsLayer(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	if _, err := a.SaveProfileGroup(devAuthBypass, profilespkg.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	for _, vid := range []string{"v1", "v2", "v3"} {
		if _, err := a.SaveProfileVariant(devAuthBypass, "g1", profilespkg.Variant{ID: vid, Name: vid}); err != nil {
			t.Fatalf("seed variant %s: %v", vid, err)
		}
	}
	if err := a.SetActiveProfile(devAuthBypass, "g1", "v2"); err != nil {
		t.Fatalf("set active: %v", err)
	}

	if err := a.DeleteProfileGroup(devAuthBypass, "g1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// All gone — group, every variant, the active pointer.
	listed, _ := a.ListProfileGroups(devAuthBypass)
	if len(listed) != 0 {
		t.Errorf("group survived delete")
	}
	active, _ := a.GetActiveProfile(devAuthBypass)
	if active.GroupID != "" {
		t.Errorf("active pointer survived cascading delete: %+v", active)
	}
}

func TestDeleteVariantNotFound(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	if _, err := a.SaveProfileGroup(devAuthBypass, profilespkg.Group{ID: "g1", Name: "G1"}); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	err := a.DeleteProfileVariant(devAuthBypass, "g1", "v-does-not-exist")
	if !errors.Is(err, profilespkg.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing variant, got %v", err)
	}
}

func TestSaveVariantRejectsUnknownGroup(t *testing.T) {
	t.Parallel()
	a := newProfilesTestApp(t)

	// Group doesn't exist for this user → the bindings must surface
	// the store's ErrNotFound, not a confusing FK or DB-level error.
	_, err := a.SaveProfileVariant(devAuthBypass, "g-not-mine", profilespkg.Variant{ID: "v1", Name: "v"})
	if !errors.Is(err, profilespkg.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown group, got %v", err)
	}
}

func TestNilStoreSurfacesUsableError(t *testing.T) {
	t.Parallel()
	// Some test fixtures build App without a DB; the methods must
	// not panic and the error text should tell the human what's up.
	logger := slog.New(slog.DiscardHandler)
	a := &App{ctx: context.Background(), logger: logger}
	// Note: no a.profiles, no a.auth. Both nil checks should fire.

	_, err := a.ListProfileGroups(devAuthBypass)
	if err == nil {
		t.Fatal("expected error from un-configured App")
	}
	if !strings.Contains(err.Error(), "store not configured") {
		t.Errorf("error doesn't mention the missing store: %v", err)
	}
}

func TestAuthGateRejectsWhenNotConfigured(t *testing.T) {
	t.Parallel()
	// App with profiles store but no auth — auth resolution must
	// fail before the store is touched.
	logger := slog.New(slog.DiscardHandler)
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	a := &App{
		ctx:      context.Background(),
		logger:   logger,
		db:       db.DB,
		profiles: profilespkg.NewStore(db.DB, logger),
	}
	// auth deliberately not set up.

	_, err = a.ListProfileGroups("any-token")
	if err == nil || !strings.Contains(err.Error(), "auth not configured") {
		t.Fatalf("expected auth-not-configured error, got %v", err)
	}
}
