package incidents

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/argues/kube-watcher/internal/sqlitedb"
)

// testStore creates a SQLite-backed incident store using a temp-dir DB.
func testStore(t *testing.T) *Store {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	dataDir := t.TempDir()
	db, err := sqlitedb.Open(dataDir, logger)
	if err != nil {
		t.Fatalf("sqlitedb.Open() failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db.DB, logger)
}

// TestNewStoreCreatesSuccessfully verifies that NewStore returns a valid store.
func TestNewStoreCreatesSuccessfully(t *testing.T) {
	store := testStore(t)
	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

// TestCreateIncidentAssignsIDAndTimestamps tests that Create assigns ID and timestamps.
func TestCreateIncidentAssignsIDAndTimestamps(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	inc, err := store.Create(ctx, "Test Alert", "critical", "alert", "Test incident", "default")
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if inc.ID == "" {
		t.Error("expected ID to be assigned, got empty string")
	}
	if inc.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if inc.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if inc.Title != "Test Alert" {
		t.Errorf("expected Title 'Test Alert', got %q", inc.Title)
	}
	if inc.Severity != "critical" {
		t.Errorf("expected Severity 'critical', got %q", inc.Severity)
	}
	if inc.Type != "alert" {
		t.Errorf("expected Type 'alert', got %q", inc.Type)
	}
	if inc.Status != "open" {
		t.Errorf("expected Status 'open', got %q", inc.Status)
	}
}

// TestCreateIncidentWithDefaultSeverity tests that Create defaults severity to "info".
func TestCreateIncidentWithDefaultSeverity(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	inc, _ := store.Create(ctx, "Test", "", "alert", "desc", "default")
	if inc.Severity != "info" {
		t.Errorf("expected default Severity 'info', got %q", inc.Severity)
	}
}

// TestCreateIncidentWithDefaultType tests that Create defaults type to "alert".
func TestCreateIncidentWithDefaultType(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	inc, _ := store.Create(ctx, "Test", "warning", "", "desc", "default")
	if inc.Type != "alert" {
		t.Errorf("expected default Type 'alert', got %q", inc.Type)
	}
}

// TestListReturnsIncidentsNewestFirst tests that List returns incidents sorted by CreatedAt descending.
func TestListReturnsIncidentsNewestFirst(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	store.Create(ctx, "Incident 1", "info", "alert", "desc1", "ns1")
	time.Sleep(20 * time.Millisecond)
	store.Create(ctx, "Incident 2", "info", "alert", "desc2", "ns1")
	time.Sleep(20 * time.Millisecond)
	store.Create(ctx, "Incident 3", "info", "alert", "desc3", "ns1")

	incidents := store.List(ctx)
	if len(incidents) != 3 {
		t.Errorf("expected 3 incidents, got %d", len(incidents))
	}
	if incidents[0].Title != "Incident 3" || incidents[1].Title != "Incident 2" || incidents[2].Title != "Incident 1" {
		t.Errorf("expected order [3, 2, 1], got [%s, %s, %s]", incidents[0].Title, incidents[1].Title, incidents[2].Title)
	}
}

// TestGetByID tests that Get returns the correct incident.
func TestGetByID(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, "Test Incident", "warning", "alert", "description", "kube-system")

	retrieved, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if retrieved.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, retrieved.ID)
	}
	if retrieved.Title != "Test Incident" {
		t.Errorf("expected Title 'Test Incident', got %q", retrieved.Title)
	}
	if retrieved.Namespace != "kube-system" {
		t.Errorf("expected Namespace 'kube-system', got %q", retrieved.Namespace)
	}
}

// TestGetNonExistentIDReturnsError tests that Get returns error for non-existent ID.
func TestGetNonExistentIDReturnsError(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected Get() to return error for non-existent ID, got nil")
	}
}

// TestUpdateChangesFields tests that Update modifies incident fields.
func TestUpdateChangesFields(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, "Test", "info", "alert", "original", "default")
	time.Sleep(20 * time.Millisecond)

	updated, err := store.Update(ctx, created.ID, "investigating", "updated description")
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	if updated.Status != "investigating" {
		t.Errorf("expected Status 'investigating', got %q", updated.Status)
	}
	if updated.Description != "updated description" {
		t.Errorf("expected Description 'updated description', got %q", updated.Description)
	}
}

// TestUpdateToResolvedSetsResolvedAt tests that Update with status="resolved" sets ResolvedAt.
func TestUpdateToResolvedSetsResolvedAt(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, "Test", "critical", "alert", "desc", "default")
	updated, _ := store.Update(ctx, created.ID, "resolved", "")

	if updated.Status != "resolved" {
		t.Errorf("expected Status 'resolved', got %q", updated.Status)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set when status='resolved'")
	}
}

// TestUpdateNonExistentIDReturnsError tests that Update returns error for non-existent ID.
func TestUpdateNonExistentIDReturnsError(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, err := store.Update(ctx, "nonexistent-id", "resolved", "desc")
	if err == nil {
		t.Fatal("expected Update() to return error for non-existent ID, got nil")
	}
}

// TestDeleteRemovesIncident tests that Delete removes an incident.
func TestDeleteRemovesIncident(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	created, _ := store.Create(ctx, "To Delete", "info", "alert", "desc", "default")

	err := store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	_, err = store.Get(ctx, created.ID)
	if err == nil {
		t.Fatal("expected Get() to fail after Delete(), but succeeded")
	}
}

// TestDeleteNonExistentIDReturnsError tests that Delete returns error for non-existent ID.
func TestDeleteNonExistentIDReturnsError(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	err := store.Delete(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected Delete() to return error for non-existent ID, got nil")
	}
}

// TestConcurrentCreatesDoNotLoseData tests that concurrent creates all succeed.
func TestConcurrentCreatesDoNotLoseData(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	numGoroutines := 10
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Create(ctx, "Concurrent Incident", "info", "alert", "desc", "default")
			if err != nil {
				errChan <- err
			}
		}()
	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Create() failed in concurrent test: %v", err)
	}

	incidents := store.List(ctx)
	if len(incidents) != numGoroutines {
		t.Errorf("expected %d incidents in List(), got %d", numGoroutines, len(incidents))
	}
}

// TestPersistenceCreateAndReload tests that incidents survive DB reopen.
func TestPersistenceCreateAndReload(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.DiscardHandler)
	ctx := context.Background()

	// Create first store and add incidents
	db1, err := sqlitedb.Open(dataDir, logger)
	if err != nil {
		t.Fatalf("sqlitedb.Open() failed: %v", err)
	}
	store1 := NewStore(db1.DB, logger)

	inc1, _ := store1.Create(ctx, "Incident A", "critical", "alert", "desc A", "ns1")
	inc2, _ := store1.Create(ctx, "Incident B", "warning", "alert", "desc B", "ns2")
	db1.Close()

	// Reopen and verify
	db2, err := sqlitedb.Open(dataDir, logger)
	if err != nil {
		t.Fatalf("sqlitedb.Open() reopen failed: %v", err)
	}
	defer db2.Close()
	store2 := NewStore(db2.DB, logger)

	incidents := store2.List(ctx)
	if len(incidents) != 2 {
		t.Errorf("expected 2 incidents loaded from SQLite, got %d", len(incidents))
	}

	r1, err := store2.Get(ctx, inc1.ID)
	if err != nil {
		t.Fatalf("failed to get incident 1 from reopened store: %v", err)
	}
	if r1.Title != "Incident A" {
		t.Errorf("expected Title 'Incident A', got %q", r1.Title)
	}

	r2, err := store2.Get(ctx, inc2.ID)
	if err != nil {
		t.Fatalf("failed to get incident 2 from reopened store: %v", err)
	}
	if r2.Title != "Incident B" {
		t.Errorf("expected Title 'Incident B', got %q", r2.Title)
	}
}
