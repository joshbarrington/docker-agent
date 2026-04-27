package builtins

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/docker/docker-agent/pkg/hooks"
)

// AddPromptFiles is the registered name of the add_prompt_files builtin.
const AddPromptFiles = "add_prompt_files"

// addPromptFiles reads each filename in args from the workdir hierarchy
// and the user's home directory, joining their contents into a
// turn_start AdditionalContext. Missing or unreadable files are logged
// and skipped; surviving files still contribute.
func addPromptFiles(_ context.Context, in *hooks.Input, args []string) (*hooks.Output, error) {
	if in == nil || in.Cwd == "" || len(args) == 0 {
		return nil, nil
	}
	var parts []string
	for _, name := range args {
		for _, path := range promptFilePaths(in.Cwd, name) {
			content, err := os.ReadFile(path)
			if err != nil {
				slog.Warn("reading prompt file", "path", path, "error", err)
				continue
			}
			parts = append(parts, string(content))
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return turnStartContext(strings.Join(parts, "\n\n")), nil
}

// promptFilePaths returns the prompt-file paths to load for filename,
// in order: the closest match found while walking up from workDir (if
// any), followed by the user's home-directory match (if it exists and
// differs from the first). Returns at most two paths.
func promptFilePaths(workDir, filename string) []string {
	var paths []string
	if p := findFileInHierarchy(workDir, filename); p != "" {
		paths = append(paths, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, filename)
		if isFile(p) && !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// findFileInHierarchy searches for filename starting at startDir and
// walking up the directory tree. Returns the path of the first match,
// or "" if none.
func findFileInHierarchy(startDir, filename string) string {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, filename)
		if isFile(path) {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// isFile reports whether path exists and is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
