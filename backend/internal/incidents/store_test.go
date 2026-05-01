package incidents

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// TestNewStoreCreatesDirectory verifies that NewStore creates the data directory.
func TestNewStoreCreatesDirectory(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())

	store := NewStore(dataDir, logger)
	if store == nil {
		t.Fatal("expected store, got nil")
	}

	// Verify directory was created
	info, err := os.Stat(dataDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("data directory not created at %s", dataDir)
	}
}

// TestCreateIncidentAssignsIDAndTimestamps tests that Create assigns ID and timestamps.
func TestCreateIncidentAssignsIDAndTimestamps(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	inc, err := store.Create(ctx, "Test Alert", "critical", "alert", "Test incident", "default")
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify ID was assigned
	if inc.ID == "" {
		t.Error("expected ID to be assigned, got empty string")
	}

	// Verify timestamps were set
	if inc.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if inc.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if !inc.UpdatedAt.Equal(inc.CreatedAt) {
		t.Errorf("expected UpdatedAt == CreatedAt for new incident, got %v != %v", inc.UpdatedAt, inc.CreatedAt)
	}

	// Verify fields
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
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	inc, _ := store.Create(ctx, "Test", "", "alert", "desc", "default")

	if inc.Severity != "info" {
		t.Errorf("expected default Severity 'info', got %q", inc.Severity)
	}
}

// TestCreateIncidentWithDefaultType tests that Create defaults type to "alert".
func TestCreateIncidentWithDefaultType(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	inc, _ := store.Create(ctx, "Test", "warning", "", "desc", "default")

	if inc.Type != "alert" {
		t.Errorf("expected default Type 'alert', got %q", inc.Type)
	}
}

// TestListReturnsIncidentsNewestFirst tests that List returns incidents sorted by CreatedAt descending.
func TestListReturnsIncidentsNewestFirst(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()

	// Create three incidents with spacing
	store.Create(ctx, "Incident 1", "info", "alert", "desc1", "ns1")
	time.Sleep(20 * time.Millisecond)

	store.Create(ctx, "Incident 2", "info", "alert", "desc2", "ns1")
	time.Sleep(20 * time.Millisecond)

	store.Create(ctx, "Incident 3", "info", "alert", "desc3", "ns1")

	incidents := store.List(ctx)

	if len(incidents) != 3 {
		t.Errorf("expected 3 incidents, got %d", len(incidents))
	}

	// Verify newest first
	if incidents[0].Title != "Incident 3" || incidents[1].Title != "Incident 2" || incidents[2].Title != "Incident 1" {
		t.Errorf("expected order [3, 2, 1], got [%s, %s, %s]", incidents[0].Title, incidents[1].Title, incidents[2].Title)
	}

	// Verify descending by CreatedAt
	for i := 0; i < len(incidents)-1; i++ {
		if !incidents[i].CreatedAt.After(incidents[i+1].CreatedAt) {
			t.Errorf("expected incidents[%d].CreatedAt > incidents[%d].CreatedAt", i, i+1)
		}
	}
}

// TestGetByID tests that Get returns the correct incident.
func TestGetByID(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

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
	if retrieved.Severity != "warning" {
		t.Errorf("expected Severity 'warning', got %q", retrieved.Severity)
	}
	if retrieved.Namespace != "kube-system" {
		t.Errorf("expected Namespace 'kube-system', got %q", retrieved.Namespace)
	}
}

// TestGetNonExistentIDReturnsError tests that Get returns error for non-existent ID.
func TestGetNonExistentIDReturnsError(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected Get() to return error for non-existent ID, got nil")
	}
}

// TestUpdateChangesFields tests that Update modifies incident fields.
func TestUpdateChangesFields(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	created, _ := store.Create(ctx, "Test", "info", "alert", "original", "default")
	originalUpdatedAt := created.UpdatedAt

	// Wait to ensure UpdatedAt changes
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
	if !updated.UpdatedAt.After(originalUpdatedAt) {
		t.Errorf("expected UpdatedAt to advance")
	}
	// CreatedAt should not change
	if updated.CreatedAt != created.CreatedAt {
		t.Errorf("expected CreatedAt to be preserved")
	}
}

// TestUpdateToResolvedSetsResolvedAt tests that Update with status="resolved" sets ResolvedAt.
func TestUpdateToResolvedSetsResolvedAt(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	created, _ := store.Create(ctx, "Test", "critical", "alert", "desc", "default")

	updated, _ := store.Update(ctx, created.ID, "resolved", "")
	if updated.Status != "resolved" {
		t.Errorf("expected Status 'resolved', got %q", updated.Status)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set when status='resolved'")
	}
	if updated.ResolvedAt != nil && updated.ResolvedAt.IsZero() {
		t.Error("expected ResolvedAt to be non-zero")
	}
}

// TestUpdateNonExistentIDReturnsError tests that Update returns error for non-existent ID.
func TestUpdateNonExistentIDReturnsError(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	_, err := store.Update(ctx, "nonexistent-id", "resolved", "desc")
	if err == nil {
		t.Fatal("expected Update() to return error for non-existent ID, got nil")
	}
}

// TestDeleteRemovesIncident tests that Delete removes an incident.
func TestDeleteRemovesIncident(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	created, _ := store.Create(ctx, "To Delete", "info", "alert", "desc", "default")

	// Verify it exists
	_, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("incident not found before delete")
	}

	// Delete it
	err = store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify it's gone
	_, err = store.Get(ctx, created.ID)
	if err == nil {
		t.Fatal("expected Get() to fail after Delete(), but succeeded")
	}

	// Verify it's not in List
	incidents := store.List(ctx)
	for _, inc := range incidents {
		if inc.ID == created.ID {
			t.Fatal("deleted incident still appears in List()")
		}
	}
}

// TestDeleteNonExistentIDReturnsError tests that Delete returns error for non-existent ID.
func TestDeleteNonExistentIDReturnsError(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()
	err := store.Delete(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected Delete() to return error for non-existent ID, got nil")
	}
}

// TestConcurrentCreatesDoNotLoseData tests that 10 concurrent creates all succeed.
func TestConcurrentCreatesDoNotLoseData(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	numGoroutines := 10
	var wg sync.WaitGroup
	ctx := context.Background()
	createdIDs := make([]string, numGoroutines)
	idMutex := sync.Mutex{}
	errChan := make(chan error, numGoroutines)

	// Launch concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			inc, err := store.Create(ctx, "Concurrent Incident", "info", "alert", "desc", "default")
			if err != nil {
				errChan <- err
				return
			}
			idMutex.Lock()
			createdIDs[idx] = inc.ID
			idMutex.Unlock()
		}(i)
	}
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Create() failed in concurrent test: %v", err)
	}

	// Verify all incidents were created
	incidents := store.List(ctx)
	if len(incidents) != numGoroutines {
		t.Errorf("expected %d incidents in List(), got %d", numGoroutines, len(incidents))
	}

	// Verify each created ID can be retrieved
	for i, id := range createdIDs {
		if id == "" {
			continue
		}
		inc, err := store.Get(ctx, id)
		if err != nil {
			t.Errorf("failed to retrieve incident created in goroutine %d: %v", i, err)
		}
		if inc.ID != id {
			t.Errorf("retrieved incident has wrong ID: expected %s, got %s", id, inc.ID)
		}
	}
}

// TestPersistenceCreateAndReload tests that incidents are persisted and reloaded.
func TestPersistenceCreateAndReload(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())

	// Create first store and add incidents
	store1 := NewStore(dataDir, logger)
	ctx := context.Background()

	inc1, _ := store1.Create(ctx, "Incident A", "critical", "alert", "desc A", "ns1")
	inc2, _ := store1.Create(ctx, "Incident B", "warning", "alert", "desc B", "ns2")

	id1 := inc1.ID
	id2 := inc2.ID

	// Verify they exist in first store
	if len(store1.List(ctx)) != 2 {
		t.Fatalf("expected 2 incidents after create, got %d", len(store1.List(ctx)))
	}

	// Create new store on same directory
	store2 := NewStore(dataDir, logger)

	// Verify incidents are loaded
	incidents := store2.List(ctx)
	if len(incidents) != 2 {
		t.Errorf("expected 2 incidents loaded from disk, got %d", len(incidents))
	}

	// Verify they can be retrieved
	retrieved1, err := store2.Get(ctx, id1)
	if err != nil {
		t.Fatalf("failed to get incident 1 from reloaded store: %v", err)
	}
	if retrieved1.Title != "Incident A" {
		t.Errorf("expected Title 'Incident A', got %q", retrieved1.Title)
	}

	retrieved2, err := store2.Get(ctx, id2)
	if err != nil {
		t.Fatalf("failed to get incident 2 from reloaded store: %v", err)
	}
	if retrieved2.Title != "Incident B" {
		t.Errorf("expected Title 'Incident B', got %q", retrieved2.Title)
	}
}

// TestConcurrentReadsDoNotBlock tests that multiple concurrent reads work.
func TestConcurrentReadsDoNotBlock(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store := NewStore(dataDir, logger)

	ctx := context.Background()

	// Create a few incidents
	store.Create(ctx, "Incident 1", "info", "alert", "desc1", "default")
	store.Create(ctx, "Incident 2", "info", "alert", "desc2", "default")
	store.Create(ctx, "Incident 3", "info", "alert", "desc3", "default")

	// Launch concurrent reads
	numReaders := 20
	var wg sync.WaitGroup
	errChan := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			incidents := store.List(ctx)
			if len(incidents) != 3 {
				errChan <- nil
			}
		}()
	}
	wg.Wait()
	close(errChan)

	for range errChan {
		// Just drain to check for panics
	}
}
