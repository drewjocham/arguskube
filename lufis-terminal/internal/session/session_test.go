package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestSaveAndLoad(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)

	state := State{
		Tabs: []Tab{
			{ID: "tab1", Title: "main", Panes: []Pane{{ID: "p1", Command: "zsh"}}},
		},
		Active: 0,
	}
	require.NoError(t, s.Save(state))

	loaded, err := s.Load()
	require.NoError(t, err)
	assert.Len(t, loaded.Tabs, 1)
	assert.Equal(t, "tab1", loaded.Tabs[0].ID)
	assert.Equal(t, "zsh", loaded.Tabs[0].Panes[0].Command)
}

func TestLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	_, err := s.Load()
	assert.Error(t, err)
}

func TestClear(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	_ = s.Save(State{})
	require.NoError(t, s.Clear())
	assert.False(t, s.Exists())
}

func TestExists(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	assert.False(t, s.Exists())
	_ = s.Save(State{})
	assert.True(t, s.Exists())
}
