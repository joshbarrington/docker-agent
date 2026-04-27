package builtins

import (
	"os"
	"path/filepath"
)

// isGitRepo checks if the given directory or one of its parents is a git
// repository. Used by the add_environment_info builtin to surface git
// awareness to the model.
func isGitRepo(dir string) bool {
	if dir == "" {
		return false
	}

	current, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	for {
		info, err := os.Stat(filepath.Join(current, ".git"))
		if err != nil {
			if !os.IsNotExist(err) {
				return false
			}
		} else if info.IsDir() {
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}
