package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/dgageot/rubocop-go/cop"
)

// ConfigVersionImport enforces that config version packages (pkg/config/vN
// and pkg/config/latest) only import their immediate predecessor and the
// shared types package, preserving the strict migration chain:
// v0 → v1 → v2 → … → latest.
type ConfigVersionImport struct{}

func (*ConfigVersionImport) Name() string { return "Lint/ConfigVersionImport" }
func (*ConfigVersionImport) Description() string {
	return "Config version packages must only import their immediate predecessor"
}
func (*ConfigVersionImport) Severity() cop.Severity { return cop.Error }

func (c *ConfigVersionImport) Check(fset *token.FileSet, file *ast.File) []cop.Offense {
	if len(file.Imports) == 0 {
		return nil
	}

	dir := configDir(fset.Position(file.Package).Filename)
	if dir == "" {
		return nil
	}
	// Black-box test files (package <dir>_test) are external to the package
	// and may import what they please.
	if strings.HasSuffix(file.Name.Name, "_test") {
		return nil
	}

	dirVersion, isVersioned := versionFromDir(dir)
	isLatest := dir == "latest"
	if !isVersioned && !isLatest {
		return nil
	}

	var offenses []cop.Offense
	for _, imp := range file.Imports {
		path := importPath(imp)

		if !strings.Contains(path, "pkg/config/") || strings.HasSuffix(path, "pkg/config/types") {
			continue
		}
		if msg := importViolation(path, dirVersion, isLatest); msg != "" {
			offenses = append(offenses, offense(c, fset, imp.Path, msg))
		}
	}
	return offenses
}

// importViolation returns a non-empty error message if the given import path
// is forbidden inside a config-version package, or "" if the import is fine.
// dirVersion is the importing package's N (only meaningful when !isLatest).
func importViolation(path string, dirVersion int, isLatest bool) string {
	if isLatest {
		// pkg/config/latest may only import other config-version packages.
		// (The "must be the immediate predecessor" rule lives in the
		// LatestImportsPredecessor cop.)
		if _, ok := versionFromImport(path); ok {
			return ""
		}
		return "pkg/config/latest must only import config version or types packages, not " + path
	}

	// Versioned package (vN).
	if strings.HasSuffix(path, "pkg/config/latest") {
		return fmt.Sprintf("config v%d must not import pkg/config/latest", dirVersion)
	}
	imported, ok := versionFromImport(path)
	if !ok {
		return ""
	}
	expected := dirVersion - 1
	if expected < 0 {
		return "config v0 must not import other config version packages"
	}
	if imported != expected {
		return fmt.Sprintf("config v%d must import v%d (its predecessor), not v%d", dirVersion, expected, imported)
	}
	return ""
}
