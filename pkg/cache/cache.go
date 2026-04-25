// Package cache implements a configurable response cache for agents.
//
// The cache maps a normalized user question to a previously produced agent
// response so that asking the exact same (according to the configured
// normalization rules) question again returns the same answer without
// invoking the model.
//
// Two storage backends are supported:
//
//   - an in-memory map (the default), which keeps entries for the lifetime of
//     the process;
//   - a JSON-file backed store, which persists entries to disk so they
//     survive restarts. Writes to the JSON file are atomic: the file is
//     written to a sibling temp file, fsync'd, and renamed over the
//     destination, so a concurrent reader (or a process that crashes
//     mid-write) will always see either the previous content or the new
//     content in full — never a partially written file.
//
// Two normalization options are exposed:
//
//   - case sensitivity: when disabled (the default), questions are
//     compared case-insensitively;
//   - blank trimming: when enabled, leading and trailing whitespace is
//     stripped before comparison.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config describes how a Cache should normalize keys and where it should
// store entries.
type Config struct {
	// Enabled toggles the cache on or off. When false, New returns nil.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// CaseSensitive controls whether the question matching is
	// case-sensitive. The default (false) means "Hello" and "hello"
	// are treated as the same question.
	CaseSensitive bool `json:"case_sensitive,omitempty" yaml:"case_sensitive,omitempty"`

	// TrimSpaces controls whether leading and trailing whitespace is
	// removed from questions before they are compared. The default
	// (false) preserves whitespace.
	TrimSpaces bool `json:"trim_spaces,omitempty" yaml:"trim_spaces,omitempty"`

	// Path, when non-empty, selects a JSON-file backed cache stored at
	// the given path. When empty, the cache lives only in memory.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// Cache stores agent responses keyed on the user's question.
//
// Implementations must be safe for concurrent use.
type Cache interface {
	// Lookup returns the stored response for the given question and a
	// boolean indicating whether the question was found.
	Lookup(question string) (string, bool)

	// Store records the response for the given question, replacing any
	// existing entry with the same normalized key.
	Store(question, response string)
}

// New builds a Cache from the given Config. It returns (nil, nil) when
// caching is disabled, allowing callers to short-circuit with a simple
// nil check.
func New(cfg Config) (Cache, error) {
	if !cfg.Enabled {
		return nil, nil //nolint:nilnil // intentional: nil signals caching disabled
	}

	c := &cache{
		entries:   make(map[string]string),
		normalize: keyNormalizer(cfg.CaseSensitive, cfg.TrimSpaces),
	}

	if cfg.Path != "" {
		if err := loadFromFile(cfg.Path, c.entries); err != nil {
			return nil, err
		}
		path := cfg.Path
		c.persist = func(snapshot map[string]string) {
			// Persistence failures must not break a successful agent
			// turn — entries remain available from memory and the next
			// Store will retry the file write.
			_ = writeJSON(path, snapshot)
		}
	}

	return c, nil
}

// cache is a single in-memory store. When persist is non-nil it is also
// invoked with a snapshot after every Store to mirror entries to durable
// storage; that keeps the in-memory and on-disk implementations as one
// piece of code with one extra optional callback.
type cache struct {
	mu        sync.RWMutex
	entries   map[string]string
	normalize func(string) string
	persist   func(map[string]string)
}

func (c *cache) Lookup(question string) (string, bool) {
	key := c.normalize(question)
	c.mu.RLock()
	defer c.mu.RUnlock()
	resp, ok := c.entries[key]
	return resp, ok
}

func (c *cache) Store(question, response string) {
	key := c.normalize(question)

	c.mu.Lock()
	c.entries[key] = response
	var snapshot map[string]string
	if c.persist != nil {
		snapshot = maps.Clone(c.entries)
	}
	c.mu.Unlock()

	if c.persist != nil {
		c.persist(snapshot)
	}
}

// keyNormalizer returns a function that applies the configured
// normalization rules to a question before it is used as a cache key.
func keyNormalizer(caseSensitive, trimSpaces bool) func(string) string {
	return func(s string) string {
		if trimSpaces {
			s = strings.TrimSpace(s)
		}
		if !caseSensitive {
			s = strings.ToLower(s)
		}
		return s
	}
}

// loadFromFile decodes path into entries. A missing file is not an error
// and leaves entries empty.
func loadFromFile(path string, entries map[string]string) error {
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("reading cache file %q: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("loading cache file %q: %w", path, err)
	}
	return nil
}

// writeJSON atomically writes entries to path as pretty-printed JSON.
//
// The new content is written to a sibling temp file, fsync'd, and renamed
// over the destination. POSIX guarantees the rename is atomic, so a
// concurrent reader sees either the previous content or the new content
// in full — never a partial write. The parent directory is fsync'd
// after the rename so the rename itself is durable across an OS crash.
func writeJSON(path string, entries map[string]string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating cache directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".cache-*.json")
	if err != nil {
		return fmt.Errorf("creating temp cache file: %w", err)
	}
	tmpName := tmp.Name()
	// Cleanup on any error path; harmless once Rename has moved the file.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp cache file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp cache file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming cache file: %w", err)
	}

	syncDir(dir)
	return nil
}

// syncDir best-effort fsyncs a directory so a recent rename inside it is
// persisted. Directory fsync is not portable (e.g. unsupported on
// Windows); a failure here does not invalidate the data.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
