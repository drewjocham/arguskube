package git

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	return dir
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(dir+"/"+name, []byte(content), 0o644))
	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "add "+name).Run()
}

func TestStatusClean(t *testing.T) {
	dir := initRepo(t)
	entries, err := Status(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestStatusModified(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "a.txt", "hello")
	_ = os.WriteFile(dir+"/a.txt", []byte("world"), 0o644)

	entries, err := Status(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
}

func TestBranches(t *testing.T) {
	dir := initRepo(t)
	branches, err := Branches(dir)
	require.NoError(t, err)
	assert.Len(t, branches, 1)
	assert.Equal(t, "master", branches[0].Name)
	assert.True(t, branches[0].IsHead)
}

func TestLog(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f1", "a")
	commitFile(t, dir, "f2", "b")

	entries, err := Log(dir, 5)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestCommit(t *testing.T) {
	dir := initRepo(t)

	entries, _ := Log(dir, 5)
	assert.Len(t, entries, 1)
	assert.Equal(t, "init", entries[0].Message)
}

func TestDiff(t *testing.T) {
	dir := initRepo(t)
	commitFile(t, dir, "f1", "hello")
	_ = os.WriteFile(dir+"/f1", []byte("world"), 0o644)

	diff, err := Diff(dir)
	require.NoError(t, err)
	assert.Contains(t, diff, "-hello")
	assert.Contains(t, diff, "+world")
}
