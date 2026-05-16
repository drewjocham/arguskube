package auth

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

func TestSetAndGet(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, s.Set("openai", "sk-test"))
	cred, ok := s.Get("openai")
	assert.True(t, ok)
	assert.Equal(t, "sk-test", cred.APIKey)
}

func TestGetNonexistent(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	_, ok := s.Get("nonexistent")
	assert.False(t, ok)
}

func TestDelete(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, s.Set("x", "key"))
	require.NoError(t, s.Delete("x"))
	_, ok := s.Get("x")
	assert.False(t, ok)
}

func TestList(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, s.Set("a", "1"))
	require.NoError(t, s.Set("b", "2"))
	assert.Len(t, s.List(), 2)
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)
	require.NoError(t, s.Set("openai", "sk-persist"))
	require.NoError(t, s.Close())

	s2, err := NewStore(dir)
	require.NoError(t, err)
	cred, ok := s2.Get("openai")
	assert.True(t, ok)
	assert.Equal(t, "sk-persist", cred.APIKey)
}
