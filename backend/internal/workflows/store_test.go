package workflows

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

// TestStoreNewCreatesDirectory verifies that New creates the workflows subdirectory.
func TestStoreNewCreatesDirectory(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())

	store, err := New(dataDir, logger)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected store, got nil")
	}

	// Check that the workflows directory was created
	workflowsDir := dataDir + "/workflows"
	info, err := os.Stat(workflowsDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("workflows directory not created at %s", workflowsDir)
	}
}

// TestSaveNewWorkflowAssignsUUID tests that saving a workflow without an ID assigns a UUID and timestamps.
func TestSaveNewWorkflowAssignsUUID(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	wf := &Workflow{
		Title: "Test Workflow",
		Steps: []Step{
			{ID: 1, Type: "trigger", Name: "Start"},
		},
	}

	saved, err := store.Save(wf)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Check that ID was assigned (non-empty, valid UUID format)
	if saved.ID == "" {
		t.Error("expected UUID to be assigned, got empty ID")
	}
	if len(saved.ID) != 36 {
		t.Errorf("expected UUID length 36, got %d", len(saved.ID))
	}

	// Check that timestamps were set
	if saved.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if saved.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if !saved.UpdatedAt.Equal(saved.CreatedAt) {
		t.Errorf("expected UpdatedAt == CreatedAt for new workflow, got %v != %v", saved.UpdatedAt, saved.CreatedAt)
	}
}

// TestSaveExistingWorkflowPreservesCreatedAt tests that updating a workflow preserves CreatedAt.
func TestSaveExistingWorkflowPreservesCreatedAt(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	wf := &Workflow{
		Title: "Original Title",
		Steps: []Step{{ID: 1, Type: "trigger", Name: "Start"}},
	}

	saved1, _ := store.Save(wf)
	originalCreatedAt := saved1.CreatedAt

	// Small delay to ensure UpdatedAt differs
	time.Sleep(10 * time.Millisecond)

	// Update the workflow
	saved1.Title = "Updated Title"
	saved2, err := store.Save(saved1)
	if err != nil {
		t.Fatalf("Save() update failed: %v", err)
	}

	if !saved2.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("expected CreatedAt to be preserved, got %v != %v", saved2.CreatedAt, originalCreatedAt)
	}
	if !saved2.UpdatedAt.After(originalCreatedAt) {
		t.Errorf("expected UpdatedAt to advance, got %v (original: %v)", saved2.UpdatedAt, originalCreatedAt)
	}
}

// TestListReturnsSortedByUpdatedAtDesc tests that List returns workflows sorted by UpdatedAt descending.
func TestListReturnsSortedByUpdatedAtDesc(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	// Create three workflows with deliberate spacing
	wf1 := &Workflow{Title: "Workflow 1", Steps: []Step{}}
	store.Save(wf1)
	time.Sleep(20 * time.Millisecond)

	wf2 := &Workflow{Title: "Workflow 2", Steps: []Step{}}
	store.Save(wf2)
	time.Sleep(20 * time.Millisecond)

	wf3 := &Workflow{Title: "Workflow 3", Steps: []Step{}}
	store.Save(wf3)

	summaries, err := store.List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Errorf("expected 3 workflows, got %d", len(summaries))
	}

	// Verify sorted order (newest first)
	if summaries[0].Title != "Workflow 3" || summaries[1].Title != "Workflow 2" || summaries[2].Title != "Workflow 1" {
		t.Errorf("expected sorted order [3, 2, 1], got [%s, %s, %s]", summaries[0].Title, summaries[1].Title, summaries[2].Title)
	}

	// Verify descending order by UpdatedAt
	for i := 0; i < len(summaries)-1; i++ {
		if !summaries[i].UpdatedAt.After(summaries[i+1].UpdatedAt) {
			t.Errorf("expected summaries[%d].UpdatedAt > summaries[%d].UpdatedAt", i, i+1)
		}
	}
}

// TestGetReturnsCorrectWorkflow tests that Get returns the correct full workflow.
func TestGetReturnsCorrectWorkflow(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	steps := []Step{
		{ID: 1, Type: "trigger", Name: "Alert", ActionType: "http"},
		{ID: 2, Type: "action", Name: "Slack", ActionType: "slack"},
	}
	wf := &Workflow{Title: "Alert Handler", Steps: steps}
	saved, _ := store.Save(wf)

	retrieved, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if retrieved.ID != saved.ID {
		t.Errorf("expected ID %s, got %s", saved.ID, retrieved.ID)
	}
	if retrieved.Title != "Alert Handler" {
		t.Errorf("expected Title 'Alert Handler', got %q", retrieved.Title)
	}
	if len(retrieved.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(retrieved.Steps))
	}
	if retrieved.Steps[1].ActionType != "slack" {
		t.Errorf("expected step 1 ActionType 'slack', got %q", retrieved.Steps[1].ActionType)
	}
}

// TestGetNonExistentIDReturnsError tests that Get returns an error for non-existent workflows.
func TestGetNonExistentIDReturnsError(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	_, err := store.Get("nonexistent-id")
	if err == nil {
		t.Fatal("expected Get() to return error for non-existent ID, got nil")
	}
}

// TestDeleteRemovesWorkflow tests that Delete removes a workflow from the store.
func TestDeleteRemovesWorkflow(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	wf := &Workflow{Title: "To Delete", Steps: []Step{}}
	saved, _ := store.Save(wf)

	// Verify it exists
	_, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("workflow not found before delete")
	}

	// Delete it
	err = store.Delete(saved.ID)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify it no longer exists
	_, err = store.Get(saved.ID)
	if err == nil {
		t.Fatal("expected Get() to fail after Delete(), but succeeded")
	}

	// Verify it's not in List
	summaries, _ := store.List()
	for _, s := range summaries {
		if s.ID == saved.ID {
			t.Fatal("deleted workflow still appears in List()")
		}
	}
}

// TestDeleteNonExistentIDDoesNotError tests that Delete doesn't error for non-existent IDs.
func TestDeleteNonExistentIDDoesNotError(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	// Should not panic or error
	err := store.Delete("nonexistent-id")
	if err != nil {
		t.Errorf("expected Delete() to not error for non-existent ID, got: %v", err)
	}
}

// TestListSummaryContainsStepCount tests that List returns correct step counts.
func TestListSummaryContainsStepCount(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	wf1 := &Workflow{
		Title: "Two Steps",
		Steps: []Step{
			{ID: 1, Type: "trigger", Name: "Start"},
			{ID: 2, Type: "action", Name: "Action"},
		},
	}
	saved1, _ := store.Save(wf1)

	wf2 := &Workflow{
		Title: "Zero Steps",
		Steps: []Step{},
	}
	saved2, _ := store.Save(wf2)

	summaries, _ := store.List()

	for _, s := range summaries {
		if s.ID == saved1.ID && s.StepCount != 2 {
			t.Errorf("expected StepCount 2 for saved1, got %d", s.StepCount)
		}
		if s.ID == saved2.ID && s.StepCount != 0 {
			t.Errorf("expected StepCount 0 for saved2, got %d", s.StepCount)
		}
	}
}

// TestConcurrentSavesDoNotCorrupt tests that concurrent saves don't lose data or corrupt files.
func TestConcurrentSavesDoNotCorrupt(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	numGoroutines := 10
	var wg sync.WaitGroup
	savedIDs := make([]string, numGoroutines)
	idMutex := sync.Mutex{}

	// Launch concurrent saves
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			wf := &Workflow{
				Title: "Concurrent Workflow",
				Steps: []Step{
					{ID: 1, Type: "trigger", Name: "Start"},
				},
			}
			saved, err := store.Save(wf)
			if err != nil {
				t.Errorf("Save() failed in goroutine %d: %v", idx, err)
				return
			}
			idMutex.Lock()
			savedIDs[idx] = saved.ID
			idMutex.Unlock()
		}(i)
	}
	wg.Wait()

	// Verify all workflows were saved and can be retrieved
	summaries, _ := store.List()
	if len(summaries) != numGoroutines {
		t.Errorf("expected %d workflows in List(), got %d", numGoroutines, len(summaries))
	}

	// Verify each saved ID can be retrieved
	for i, id := range savedIDs {
		if id == "" {
			continue
		}
		wf, err := store.Get(id)
		if err != nil {
			t.Errorf("failed to retrieve workflow saved in goroutine %d: %v", i, err)
		}
		if wf.ID != id {
			t.Errorf("retrieved workflow has wrong ID: expected %s, got %s", id, wf.ID)
		}
	}
}

// TestConcurrentReadsDoNotBlock tests that multiple concurrent reads work without blocking.
func TestConcurrentReadsDoNotBlock(t *testing.T) {
	dataDir := t.TempDir()
	logger := slog.New(slog.NewDiscardHandler())
	store, _ := New(dataDir, logger)

	// Create a workflow
	wf := &Workflow{Title: "Read Test", Steps: []Step{}}
	saved, _ := store.Save(wf)

	// Launch concurrent reads
	numReaders := 20
	var wg sync.WaitGroup
	errChan := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Get(saved.ID)
			if err != nil {
				errChan <- err
			}
			_, err = store.List()
			if err != nil {
				errChan <- err
			}
		}()
	}
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("concurrent read failed: %v", err)
	}
}
