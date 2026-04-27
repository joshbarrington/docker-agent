package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_disabled(t *testing.T) {
	c, err := New(Config{Enabled: false})
	require.NoError(t, err)
	assert.Nil(t, c)
}

func TestMemoryCache_caseSensitiveDefault(t *testing.T) {
	c, err := New(Config{Enabled: true, CaseSensitive: true})
	require.NoError(t, err)
	require.NotNil(t, c)

	c.Store("Hello", "world")

	got, ok := c.Lookup("Hello")
	assert.True(t, ok)
	assert.Equal(t, "world", got)

	_, ok = c.Lookup("hello")
	assert.False(t, ok, "case-sensitive cache should not match different case")
}

func TestMemoryCache_caseInsensitive(t *testing.T) {
	c, err := New(Config{Enabled: true, CaseSensitive: false})
	require.NoError(t, err)

	c.Store("Hello", "world")

	got, ok := c.Lookup("HELLO")
	assert.True(t, ok)
	assert.Equal(t, "world", got)
}

func TestMemoryCache_trimSpaces(t *testing.T) {
	c, err := New(Config{Enabled: true, TrimSpaces: true})
	require.NoError(t, err)

	c.Store("  hello  ", "world")

	got, ok := c.Lookup("hello")
	assert.True(t, ok)
	assert.Equal(t, "world", got)

	got, ok = c.Lookup("\thello\n")
	assert.True(t, ok)
	assert.Equal(t, "world", got)
}

func TestMemoryCache_noTrimByDefault(t *testing.T) {
	c, err := New(Config{Enabled: true})
	require.NoError(t, err)

	c.Store("  hello  ", "world")

	_, ok := c.Lookup("hello")
	assert.False(t, ok, "without TrimSpaces, whitespace must be significant")
}

func TestMemoryCache_overwrite(t *testing.T) {
	c, err := New(Config{Enabled: true})
	require.NoError(t, err)

	c.Store("q", "first")
	c.Store("q", "second")

	got, ok := c.Lookup("q")
	assert.True(t, ok)
	assert.Equal(t, "second", got)
}

// TestFileCache_dedupSkipsRedundantWrite verifies that storing the exact
// same (question, response) pair twice is treated as a no-op, so the
// underlying JSON file is rewritten only on the first Store. This is
// what keeps cache replays free of redundant disk traffic.
func TestFileCache_dedupSkipsRedundantWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c, err := New(Config{Enabled: true, Path: path})
	require.NoError(t, err)

	c.Store("q", "a")
	infoBefore, err := os.Stat(path)
	require.NoError(t, err)

	// Same pair: must not rewrite the file (mtime stays the same).
	c.Store("q", "a")
	infoAfter, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime(),
		"identical Store must not rewrite the cache file")

	// Different value: must rewrite.
	c.Store("q", "b")
	infoChanged, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, infoChanged.ModTime().After(infoBefore.ModTime()) || infoChanged.Size() != infoBefore.Size(),
		"different Store must rewrite the cache file")
}

func TestFileCache_persists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c1, err := New(Config{Enabled: true, Path: path, CaseSensitive: false, TrimSpaces: true})
	require.NoError(t, err)
	c1.Store("  Hello  ", "world")

	// File must exist on disk and contain the normalized key.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var entries map[string]string
	require.NoError(t, json.Unmarshal(data, &entries))
	assert.Equal(t, map[string]string{"hello": "world"}, entries)

	// A new cache loaded from the same file recovers the entries.
	c2, err := New(Config{Enabled: true, Path: path, CaseSensitive: false, TrimSpaces: true})
	require.NoError(t, err)
	got, ok := c2.Lookup("HELLO")
	assert.True(t, ok)
	assert.Equal(t, "world", got)
}

func TestFileCache_missingFileIsFine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "cache.json")

	c, err := New(Config{Enabled: true, Path: path})
	require.NoError(t, err)

	_, ok := c.Lookup("anything")
	assert.False(t, ok)

	c.Store("hello", "world")

	got, ok := c.Lookup("hello")
	assert.True(t, ok)
	assert.Equal(t, "world", got)

	// And the directory should have been created.
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestFileCache_corruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	_, err := New(Config{Enabled: true, Path: path})
	assert.Error(t, err)
}

// TestFileCache_atomicWriteLeavesNoTempFiles verifies that the rename-based
// atomic write does not leak temporary files on the happy path.
func TestFileCache_atomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c, err := New(Config{Enabled: true, Path: path})
	require.NoError(t, err)

	for i := range 5 {
		c.Store(fmt.Sprintf("q%d", i), fmt.Sprintf("a%d", i))
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		assert.Equal(t, "cache.json", e.Name(),
			"unexpected leftover in cache directory: %q", e.Name())
	}
}

// TestFileCache_concurrentStoreNeverYieldsTornFile verifies that concurrent
// Store calls always leave a fully valid JSON file behind — i.e. a parallel
// reader will never observe a half-written cache thanks to the
// rename-over-temp atomicity.
func TestFileCache_concurrentStoreNeverYieldsTornFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c, err := New(Config{Enabled: true, Path: path})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 50 {
			c.Store(fmt.Sprintf("q%d", i), fmt.Sprintf("a%d", i))
		}
	}()

	// While writes are happening, repeatedly read and parse the file.
	// Without atomic rename, this would intermittently see truncated /
	// half-written content and json.Unmarshal would error.
	for range 100 {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		require.NoError(t, err)
		if len(data) == 0 {
			continue
		}
		var m map[string]string
		require.NoError(t, json.Unmarshal(data, &m),
			"reader observed a torn write: %q", string(data))
	}

	<-done
}
