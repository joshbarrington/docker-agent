package main

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dgageot/rubocop-go/cop"
)

// Helpers shared by the cops in this package. They centralise the parsing of
// pkg/config/<dir>/ paths and pkg/config/vN import paths, so each cop can
// focus on its rule rather than on regular expressions.

// configDirRe matches files under pkg/config/<dir>/. The captured group is
// the directory name immediately under pkg/config/. It accepts both
// absolute and relative paths.
var configDirRe = regexp.MustCompile(`(?:^|/)pkg/config/([^/]+)/[^/]+\.go$`)

// configDir returns the directory name immediately under pkg/config/ for
// filename, or "" if filename is not under pkg/config/<dir>/.
func configDir(filename string) string {
	m := configDirRe.FindStringSubmatch(filepath.ToSlash(filename))
	if m == nil {
		return ""
	}
	return m[1]
}

// versionFromDir parses a "vN" directory name and returns N. Returns false
// for any other name (latest, types, vendor, ...).
func versionFromDir(dir string) (int, bool) {
	if len(dir) < 2 || dir[0] != 'v' {
		return 0, false
	}
	n, err := strconv.Atoi(dir[1:])
	if err != nil {
		return 0, false
	}
	return n, true
}

// versionedImportRe matches a versioned config import path of the form
// ".../pkg/config/vN" at the end of an import path.
var versionedImportRe = regexp.MustCompile(`pkg/config/v(\d+)$`)

// versionFromImport returns N if importPath ends with "pkg/config/vN".
func versionFromImport(importPath string) (int, bool) {
	m := versionedImportRe.FindStringSubmatch(importPath)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		// Should never happen since regex only captures digits.
		return 0, false
	}
	return n, true
}

// importPath returns the unquoted import path of an ImportSpec.
func importPath(imp *ast.ImportSpec) string {
	return strings.Trim(imp.Path.Value, `"`)
}

// offense builds an Offense covering the source span of node n.
func offense(c cop.Cop, fset *token.FileSet, n ast.Node, message string) cop.Offense {
	return cop.NewOffense(c, fset, n.Pos(), n.End(), message)
}

// highestSiblingVersion returns the largest N such that pkg/config/vN/
// exists as a sibling of filename's directory. Result is cached per
// parent directory so callers can invoke it once per file cheaply.
func highestSiblingVersion(filename string) (int, bool) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return 0, false
	}
	parent := filepath.Dir(filepath.Dir(abs))

	if v, ok := highestCache.Load(parent); ok {
		n := v.(int)
		return n, n >= 0
	}
	n := scanHighestVN(parent)
	highestCache.Store(parent, n)
	return n, n >= 0
}

// highestCache memoises highestSiblingVersion. Value is -1 when no vN/
// directory exists under the key.
var highestCache sync.Map

func scanHighestVN(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return -1
	}
	highest := -1
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, ok := versionFromDir(e.Name()); ok && n > highest {
			highest = n
		}
	}
	return highest
}
