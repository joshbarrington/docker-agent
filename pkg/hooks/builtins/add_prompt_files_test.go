package builtins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// homeFixture creates filename in the user's home directory with the
// given body, returning the absolute path. If the file already existed,
// it is left in place; otherwise it is removed at test cleanup. The
// test is skipped when the home directory isn't writable.
func homeFixture(t *testing.T, filename, body string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	path := filepath.Join(home, filename)
	_, existed := os.Stat(path)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Skip("Cannot write to home directory")
	}
	if existed != nil {
		t.Cleanup(func() { os.Remove(path) })
	}
	return path
}

func TestPromptFilePathsInWorkDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filename := "test_prompt_unique_12345.md"
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))

	assert.Equal(t, []string{path}, promptFilePaths(dir, filename))
}

func TestPromptFilePathsInParent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filename := "test_prompt_parent_12345.md"
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))

	child := filepath.Join(dir, "child")
	require.NoError(t, os.Mkdir(child, 0o755))

	assert.Equal(t, []string{path}, promptFilePaths(child, filename))
}

func TestPromptFilePathsClosestMatchWins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filename := "test_prompt_readfirst_12345.md"
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte("parent"), 0o644))

	child := filepath.Join(dir, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	childPath := filepath.Join(child, filename)
	require.NoError(t, os.WriteFile(childPath, []byte("child"), 0o644))

	assert.Equal(t, []string{childPath}, promptFilePaths(child, filename))
}

func TestPromptFilePathsNoMatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filename := "test_prompt_nonexistent_12345.md"

	assert.Empty(t, promptFilePaths(dir, filename))
}

func TestPromptFilePathsWorkDirAndHome(t *testing.T) {
	t.Parallel()

	filename := "test_prompt_workdir_and_home_12345.md"
	homePath := homeFixture(t, filename, "home content")

	workDir := t.TempDir()
	workDirPath := filepath.Join(workDir, filename)
	require.NoError(t, os.WriteFile(workDirPath, []byte("workdir content"), 0o644))

	assert.Equal(t, []string{workDirPath, homePath}, promptFilePaths(workDir, filename))
}

func TestPromptFilePathsHomeOnly(t *testing.T) {
	t.Parallel()

	filename := "test_prompt_home_only_12345.md"
	homePath := homeFixture(t, filename, "home content")

	workDir := t.TempDir()

	assert.Equal(t, []string{homePath}, promptFilePaths(workDir, filename))
}

func TestPromptFilePathsDeduplicateHomeAndWorkDir(t *testing.T) {
	t.Parallel()

	filename := "test_prompt_dedup_12345.md"
	homePath := homeFixture(t, filename, "home content")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	// Working directory == home: the home match must not be appended a
	// second time.
	assert.Equal(t, []string{homePath}, promptFilePaths(home, filename))
}
