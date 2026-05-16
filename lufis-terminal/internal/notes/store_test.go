package notes

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	note := Note{
		ID:        "test-1",
		Command:   "kubectl get pods",
		ExitCode:  0,
		Timestamp: time.Now(),
		Body:      "All pods running",
		Tags:      []string{"k8s", "deploy"},
	}

	err = s.Save(note)
	require.NoError(t, err)

	got, err := s.Get("test-1")
	require.NoError(t, err)
	assert.Equal(t, "kubectl get pods", got.Command)
	assert.Equal(t, "All pods running", got.Body)
	assert.Equal(t, []string{"k8s", "deploy"}, got.Tags)
}

func TestStoreGetNotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	_, err = s.Get("nonexistent")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	n := Note{ID: "del-test", Body: "delete me", Timestamp: time.Now()}
	require.NoError(t, s.Save(n))
	require.NoError(t, s.Delete("del-test"))

	_, err = s.Get("del-test")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	_ = s.Save(Note{ID: "a", Body: "alpha", Tags: []string{"k8s"}, Timestamp: time.Now()})
	_ = s.Save(Note{ID: "b", Body: "beta", Tags: []string{"db"}, Timestamp: time.Now().Add(-time.Hour)})

	all := s.List(NoteFilter{})
	assert.Len(t, all, 2)

	filtered := s.List(NoteFilter{Tag: "k8s"})
	assert.Len(t, filtered, 1)
	assert.Equal(t, "alpha", filtered[0].Body)
}

func TestStoreSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	_ = s.Save(Note{ID: "1", Body: "deploy to production", Timestamp: time.Now()})
	_ = s.Save(Note{ID: "2", Body: "rollback staging", Timestamp: time.Now()})

	results := s.Search("deploy")
	assert.Len(t, results, 1)

	results = s.Search("staging")
	assert.Len(t, results, 1)

	results = s.Search("nonexistent")
	assert.Len(t, results, 0)
}

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	_ = s.Save(Note{ID: "persist", Body: "survives restart", Timestamp: time.Now()})
	s.Close()

	s2, err := NewStore(dir)
	require.NoError(t, err)
	got, err := s2.Get("persist")
	require.NoError(t, err)
	assert.Equal(t, "survives restart", got.Body)
}

func TestStoreAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	_ = s.Save(Note{ID: "atomic", Body: "data safety", Timestamp: time.Now()})

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp")
	}
}

func TestNoteFilterSession(t *testing.T) {
	now := time.Now()
	notes := []Note{
		{ID: "1", SessionID: "sess-a", Timestamp: now},
		{ID: "2", SessionID: "sess-b", Timestamp: now},
	}

	filter := NoteFilter{SessionID: "sess-a"}
	for _, n := range notes {
		assert.True(t, filter.Match(&n) == (n.SessionID == "sess-a"))
	}
}

func TestNoteFilterSince(t *testing.T) {
	now := time.Now()
	notes := []Note{
		{ID: "old", Timestamp: now.Add(-24 * time.Hour)},
		{ID: "new", Timestamp: now},
	}

	filter := NoteFilter{Since: now.Add(-time.Hour)}
	assert.False(t, filter.Match(&notes[0]))
	assert.True(t, filter.Match(&notes[1]))
}

func TestDefaultDataDir(t *testing.T) {
	s, err := NewStore("")
	require.NoError(t, err)
	assert.Contains(t, s.path, "argus-terminal")
	assert.Contains(t, s.path, "notes.json")
}

func TestConcurrentSave(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = s.Save(Note{ID: fmt.Sprintf("concurrent-%d", i), Body: "test", Timestamp: time.Now()})
		}(i)
	}
	wg.Wait()

	all := s.List(NoteFilter{})
	assert.Len(t, all, 10)
}
