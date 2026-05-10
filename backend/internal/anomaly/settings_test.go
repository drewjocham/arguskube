package anomaly_test

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/argues/kube-watcher/internal/anomaly"
	_ "modernc.org/sqlite"
)

// testSettingsStore creates a SettingsStore backed by an in-memory SQLite database.
func testSettingsStore(t *testing.T) *anomaly.SettingsStore {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store, err := anomaly.NewSettingsStore(db, logger)
	if err != nil {
		t.Fatalf("NewSettingsStore() failed: %v", err)
	}
	return store
}

func TestNewSettingsStore(t *testing.T) {
	store := testSettingsStore(t)
	if store == nil {
		t.Fatal("NewSettingsStore() returned nil")
	}
}

func TestGetSettingsDefaults(t *testing.T) {
	store := testSettingsStore(t)

	settings := store.GetSettings()
	if settings.Sensitivity != 30 {
		t.Errorf("expected default Sensitivity 30, got %d", settings.Sensitivity)
	}
	if settings.BaselineWindow != 7 {
		t.Errorf("expected default BaselineWindow 7, got %d", settings.BaselineWindow)
	}
	if settings.MetricType != "cpu" {
		t.Errorf("expected default MetricType 'cpu', got %q", settings.MetricType)
	}
	if settings.Algorithm != "smart" {
		t.Errorf("expected default Algorithm 'smart', got %q", settings.Algorithm)
	}
	if settings.Threshold != 85 {
		t.Errorf("expected default Threshold 85, got %d", settings.Threshold)
	}
	if settings.TargetScope != "all" {
		t.Errorf("expected default TargetScope 'all', got %q", settings.TargetScope)
	}
}

func TestSaveAndGetSettings(t *testing.T) {
	store := testSettingsStore(t)

	settings := anomaly.Settings{
		Sensitivity:    75,
		BaselineWindow: 14,
		MetricType:     "mem",
		Algorithm:      "fixed",
		Threshold:      90,
		TargetScope:    "production",
	}

	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings() returned error: %v", err)
	}

	got := store.GetSettings()
	if got.Sensitivity != 75 {
		t.Errorf("expected Sensitivity 75, got %d", got.Sensitivity)
	}
	if got.BaselineWindow != 14 {
		t.Errorf("expected BaselineWindow 14, got %d", got.BaselineWindow)
	}
	if got.MetricType != "mem" {
		t.Errorf("expected MetricType 'mem', got %q", got.MetricType)
	}
	if got.Algorithm != "fixed" {
		t.Errorf("expected Algorithm 'fixed', got %q", got.Algorithm)
	}
	if got.Threshold != 90 {
		t.Errorf("expected Threshold 90, got %d", got.Threshold)
	}
	if got.TargetScope != "production" {
		t.Errorf("expected TargetScope 'production', got %q", got.TargetScope)
	}
}

func TestSaveSettingsOverwrites(t *testing.T) {
	store := testSettingsStore(t)

	// Save first settings.
	_ = store.SaveSettings(anomaly.Settings{
		Sensitivity:    50,
		BaselineWindow: 7,
		MetricType:     "cpu",
		Algorithm:      "smart",
		Threshold:      80,
		TargetScope:    "staging",
	})

	// Overwrite with new settings.
	_ = store.SaveSettings(anomaly.Settings{
		Sensitivity:    90,
		BaselineWindow: 30,
		MetricType:     "mem",
		Algorithm:      "fixed",
		Threshold:      95,
		TargetScope:    "production",
	})

	got := store.GetSettings()
	if got.Sensitivity != 90 {
		t.Errorf("expected Sensitivity 90, got %d", got.Sensitivity)
	}
}

func TestListRulesDefaultSeeded(t *testing.T) {
	store := testSettingsStore(t)

	rules := store.ListRules()
	if len(rules) != 4 {
		t.Fatalf("expected 4 default rules, got %d", len(rules))
	}

	expectedRules := map[string]bool{
		"rule-1": true,
		"rule-2": true,
		"rule-3": false,
		"rule-4": true,
	}

	for _, r := range rules {
		expectedEnabled, ok := expectedRules[r.ID]
		if !ok {
			t.Errorf("unexpected rule ID: %s", r.ID)
			continue
		}
		if r.Enabled != expectedEnabled {
			t.Errorf("rule %s: expected enabled=%v, got %v", r.ID, expectedEnabled, r.Enabled)
		}
		if r.Severity == "" {
			t.Errorf("rule %s: expected non-empty severity", r.ID)
		}
		if r.Name == "" {
			t.Errorf("rule %s: expected non-empty name", r.ID)
		}
	}
}

func TestListRulesEmptyAfterDelete(t *testing.T) {
	store := testSettingsStore(t)

	// Delete all default rules.
	rules := store.ListRules()
	for _, r := range rules {
		if err := store.DeleteRule(r.ID); err != nil {
			t.Fatalf("DeleteRule(%q) returned error: %v", r.ID, err)
		}
	}

	results := store.ListRules()
	if len(results) != 0 {
		t.Errorf("expected 0 rules after deleting all, got %d", len(results))
	}
}

func TestSaveRule(t *testing.T) {
	store := testSettingsStore(t)

	rule := anomaly.Rule{
		ID:       "rule-test-1",
		Name:     "Custom Rule",
		Enabled:  true,
		Severity: "critical",
	}

	if err := store.SaveRule(rule); err != nil {
		t.Fatalf("SaveRule() returned error: %v", err)
	}

	rules := store.ListRules()
	found := false
	for _, r := range rules {
		if r.ID == "rule-test-1" {
			found = true
			if r.Name != "Custom Rule" {
				t.Errorf("expected Name 'Custom Rule', got %q", r.Name)
			}
			if !r.Enabled {
				t.Error("expected Enabled = true")
			}
			if r.Severity != "critical" {
				t.Errorf("expected Severity 'critical', got %q", r.Severity)
			}
			break
		}
	}
	if !found {
		t.Error("custom rule not found in ListRules()")
	}
}

func TestToggleRule(t *testing.T) {
	store := testSettingsStore(t)

	// Toggle rule-1 (default enabled=true) to disabled.
	newState, err := store.ToggleRule("rule-1")
	if err != nil {
		t.Fatalf("ToggleRule() returned error: %v", err)
	}
	if newState {
		t.Error("expected rule to be disabled after toggle")
	}

	// Confirm from ListRules.
	rules := store.ListRules()
	for _, r := range rules {
		if r.ID == "rule-1" && r.Enabled {
			t.Error("rule-1 should be disabled after toggle")
		}
	}

	// Toggle again — should be enabled.
	newState, err = store.ToggleRule("rule-1")
	if err != nil {
		t.Fatalf("ToggleRule() returned error: %v", err)
	}
	if !newState {
		t.Error("expected rule to be enabled after second toggle")
	}
}

func TestToggleNonexistentRule(t *testing.T) {
	store := testSettingsStore(t)

	_, err := store.ToggleRule("nonexistent-rule")
	if err == nil {
		t.Fatal("expected error for toggling nonexistent rule")
	}
}

func TestDeleteRule(t *testing.T) {
	store := testSettingsStore(t)

	if err := store.DeleteRule("rule-1"); err != nil {
		t.Fatalf("DeleteRule() returned error: %v", err)
	}

	rules := store.ListRules()
	for _, r := range rules {
		if r.ID == "rule-1" {
			t.Error("rule-1 should not exist after deletion")
		}
	}
}

func TestDeleteNonexistentRule(t *testing.T) {
	store := testSettingsStore(t)

	err := store.DeleteRule("nonexistent-rule")
	if err != nil {
		t.Errorf("deleting nonexistent rule should not error, got: %v", err)
	}
}

func TestSaveRuleOverwrites(t *testing.T) {
	store := testSettingsStore(t)

	// Save with initial values.
	_ = store.SaveRule(anomaly.Rule{
		ID:       "rule-test-overwrite",
		Name:     "Original Name",
		Enabled:  true,
		Severity: "info",
	})

	// Overwrite with new values.
	_ = store.SaveRule(anomaly.Rule{
		ID:       "rule-test-overwrite",
		Name:     "Updated Name",
		Enabled:  false,
		Severity: "critical",
	})

	rules := store.ListRules()
	for _, r := range rules {
		if r.ID == "rule-test-overwrite" {
			if r.Name != "Updated Name" {
				t.Errorf("expected Name 'Updated Name', got %q", r.Name)
			}
			if r.Enabled {
				t.Error("expected Enabled = false after overwrite")
			}
			if r.Severity != "critical" {
				t.Errorf("expected Severity 'critical', got %q", r.Severity)
			}
			return
		}
	}
	t.Error("overwritten rule not found")
}

func TestSettingsCorruptJSON(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() failed: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store, err := anomaly.NewSettingsStore(db, logger)
	if err != nil {
		t.Fatalf("NewSettingsStore() failed: %v", err)
	}

	// Insert corrupt JSON directly.
	_, err = db.Exec("INSERT INTO anomaly_settings (key, value) VALUES ('config', '{invalid json}')")
	if err != nil {
		t.Fatalf("failed to insert corrupt data: %v", err)
	}

	// Should return default settings without panicking.
	settings := store.GetSettings()
	if settings.Sensitivity != 30 {
		t.Errorf("expected default Sensitivity 30 after corrupt data, got %d", settings.Sensitivity)
	}
}

func TestNewSettingsStoreBadDB(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	_, err := anomaly.NewSettingsStore(nil, logger)
	if err == nil {
		t.Fatal("expected error with nil db")
	}
}

func TestNewSettingsStoreLogsSeeded(t *testing.T) {
	// Test that the store can be created multiple times without re-seeding issues.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() failed: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store1, err := anomaly.NewSettingsStore(db, logger)
	if err != nil {
		t.Fatalf("first NewSettingsStore() failed: %v", err)
	}

	rules1 := store1.ListRules()
	if len(rules1) != 4 {
		t.Fatalf("expected 4 rules after first create, got %d", len(rules1))
	}

	// Create again with same DB — should not re-seed.
	store2, err := anomaly.NewSettingsStore(db, logger)
	if err != nil {
		t.Fatalf("second NewSettingsStore() failed: %v", err)
	}

	rules2 := store2.ListRules()
	if len(rules2) != 4 {
		t.Errorf("expected 4 rules after second create (no duplicate seeds), got %d", len(rules2))
	}
}
