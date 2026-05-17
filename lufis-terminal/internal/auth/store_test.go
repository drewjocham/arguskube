package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	// MockInit replaces the platform keychain with an in-memory map for the
	// duration of the test binary. This keeps tests hermetic — no system
	// prompts, no leftover entries — and lets them run unchanged in CI.
	keyring.MockInit()
	os.Exit(m.Run())
}

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

	list := s.List()
	assert.Len(t, list, 2)
	// List must not return secret material.
	for _, c := range list {
		assert.Empty(t, c.APIKey, "List() must not leak APIKey — use Get(service)")
		assert.Empty(t, c.Token, "List() must not leak Token — use Get(service)")
	}
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

// TestOnDiskFileNeverContainsSecret is the regression guard for the whole
// PR: even with the metadata file readable, the secret must not be present.
func TestOnDiskFileNeverContainsSecret(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	require.NoError(t, err)
	require.NoError(t, s.Set("openai", "sk-this-must-not-appear"))
	require.NoError(t, s.Close())

	data, err := os.ReadFile(filepath.Join(dir, "auth.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sk-this-must-not-appear",
		"on-disk file leaked the secret — keychain integration is broken")

	// The metadata file should still be valid JSON containing the service entry.
	var metas []storedMeta
	require.NoError(t, json.Unmarshal(data, &metas))
	require.Len(t, metas, 1)
	assert.Equal(t, "openai", metas[0].Service)
	assert.True(t, metas[0].Connected)
}

func TestEmptyServiceRejected(t *testing.T) {
	s, err := NewStore(t.TempDir())
	require.NoError(t, err)
	err = s.Set("", "sk-test")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "service"))
}
