package blocks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestSaveAndGet(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	b := s.Save("echo hello", 0, "hello\n", time.Second)
	assert.NotEmpty(t, b.ID)
	assert.Equal(t, "echo hello", b.Command)
	assert.Equal(t, 0, b.ExitCode)

	got := s.Get(b.ID)
	assert.Equal(t, "echo hello", got.Command)
}

func TestRerun(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	b := s.Save("kubectl get pods", 0, "pod-a\npod-b", time.Second)
	assert.Len(t, b.Output, 1)

	s.Rerun(b.ID, "pod-c\npod-d", 0, 2*time.Second)
	assert.Len(t, s.Get(b.ID).Output, 2)
}

func TestEdit(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	b := s.Save("old command", 0, "", time.Second)

	s.Edit(b.ID, "new command")
	assert.Equal(t, "new command", s.Get(b.ID).Command)
}

func TestList(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	s.Save("cmd1", 0, "", time.Second)
	s.Save("cmd2", 0, "", time.Second)
	s.Save("cmd3", 0, "", time.Second)

	list := s.List(2)
	assert.Len(t, list, 2)
	assert.Equal(t, "cmd2", list[0].Command)
	assert.Equal(t, "cmd3", list[1].Command)
}

func TestDelete(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	b := s.Save("to-delete", 0, "", time.Second)
	s.Delete(b.ID)
	assert.Nil(t, s.Get(b.ID))
}

func TestSearch(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	s.Save("deploy to prod", 0, "", time.Second)
	s.Save("rollback staging", 0, "", time.Second)

	results := s.Search("deploy")
	assert.Len(t, results, 1)
	assert.Equal(t, "deploy to prod", results[0].Command)

	results = s.Search("staging")
	assert.Len(t, results, 1)

	results = s.Search("nonexistent")
	assert.Len(t, results, 0)
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	s.Save("persist-me", 0, "", time.Second)

	s2, _ := NewStore(dir)
	list := s2.List(10)
	assert.Len(t, list, 1)
	assert.Equal(t, "persist-me", list[0].Command)
}

func TestDeleteNonexistent(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	s.Delete("nonexistent")
}
