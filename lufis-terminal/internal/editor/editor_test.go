package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	b := NewBuffer()
	assert.Nil(t, b.Active())
	assert.Empty(t, b.Tabs())
}

func TestOpenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello\nworld"), 0o644))

	b := NewBuffer()
	tab, err := b.Open(path)
	require.NoError(t, err)
	assert.Equal(t, "test.txt", tab.Name)
	assert.Equal(t, 2, len(tab.Content))
	assert.False(t, tab.Dirty)
}

func TestOpenSameFileReturnsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("a"), 0o644)

	b := NewBuffer()
	b.Open(path)
	b.Open(path)
	assert.Len(t, b.Tabs(), 1)
}

func TestCloseTab(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.txt")
	p2 := filepath.Join(dir, "b.txt")
	os.WriteFile(p1, []byte("a"), 0o644)
	os.WriteFile(p2, []byte("b"), 0o644)

	b := NewBuffer()
	b.Open(p1)
	b.Open(p2)
	assert.Len(t, b.Tabs(), 2)

	b.Close(0)
	assert.Len(t, b.Tabs(), 1)
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("original"), 0o644)

	b := NewBuffer()
	b.Open(path)
	tab := b.Active()
	tab.Content[0] = "modified"
	tab.Dirty = true

	require.NoError(t, b.Save())
	data, _ := os.ReadFile(path)
	assert.Equal(t, "modified", string(data))
	assert.False(t, tab.Dirty)
}

func TestInsertAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("hello"), 0o644)

	b := NewBuffer()
	b.Open(path)
	b.InsertAt(" world", 0, 5)
	assert.Equal(t, "hello world", b.Active().Content[0])
	assert.True(t, b.Active().Dirty)
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("abc\ndef\nghi"), 0o644)

	b := NewBuffer()
	b.Open(path)
	results := b.Search("def")
	assert.Len(t, results, 1)
	assert.Equal(t, 1, results[0].Line)
}

func TestSyntaxHighlightGo(t *testing.T) {
	tokens := SyntaxHighlight("go")
	assert.Equal(t, "keyword", tokens["func"])
	assert.Equal(t, "keyword", tokens["return"])
	_, ok := tokens["resource"]
	assert.False(t, ok)
}

func TestSyntaxHighlightPython(t *testing.T) {
	tokens := SyntaxHighlight("py")
	assert.Equal(t, "keyword", tokens["def"])
	assert.Equal(t, "constant", tokens["True"])
}
