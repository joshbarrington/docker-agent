package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/dgageot/rubocop-go/cop"
)

// LatestImportsPredecessor enforces that files under pkg/config/latest/ that
// import a historical config version package (pkg/config/vN) only ever
// import the immediate predecessor: the highest vN under pkg/config/.
//
// The Lint/ConfigVersionImport cop verifies that *numbered* versions follow
// the v0 → v1 → v2 → … chain but accepts any vN inside pkg/config/latest/.
// This cop closes that gap so pkg/config/latest also obeys the chain
// (latest imports the highest vN, never an older version), which is required
// for the upgrade pipeline to reach the latest schema in a single hop.
type LatestImportsPredecessor struct{}

func (*LatestImportsPredecessor) Name() string { return "Lint/LatestImportsPredecessor" }
func (*LatestImportsPredecessor) Description() string {
	return "pkg/config/latest must only import its immediate predecessor (highest vN)"
}
func (*LatestImportsPredecessor) Severity() cop.Severity { return cop.Error }

func (c *LatestImportsPredecessor) Check(fset *token.FileSet, file *ast.File) []cop.Offense {
	if len(file.Imports) == 0 {
		return nil
	}
	filename := fset.Position(file.Package).Filename
	if configDir(filename) != "latest" {
		return nil
	}
	// Black-box test files (package latest_test) are external to the package
	// and may import what they please.
	if strings.HasSuffix(file.Name.Name, "_test") {
		return nil
	}
	highest, ok := highestSiblingVersion(filename)
	if !ok {
		return nil
	}

	var offenses []cop.Offense
	for _, imp := range file.Imports {
		got, ok := versionFromImport(importPath(imp))
		if !ok || got == highest {
			continue
		}
		offenses = append(offenses, offense(c, fset, imp.Path,
			fmt.Sprintf("pkg/config/latest must import its predecessor v%d, not v%d", highest, got)))
	}
	return offenses
}
