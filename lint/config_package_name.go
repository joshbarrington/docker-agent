package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/dgageot/rubocop-go/cop"
)

// ConfigPackageName enforces that files under pkg/config/<dir>/ declare a
// package name that matches their directory:
//
//   - pkg/config/vN/      → package vN
//   - pkg/config/latest/  → package latest
//   - pkg/config/types/   → package types
//
// This catches a class of copy-paste bugs that occur when a "latest" version
// is frozen into a numbered vN directory: the package clause is easy to
// forget, and the broken state remains compilable as long as importers use
// an explicit alias.
type ConfigPackageName struct{}

func (*ConfigPackageName) Name() string { return "Lint/ConfigPackageName" }
func (*ConfigPackageName) Description() string {
	return "Files under pkg/config/<dir>/ must declare package <dir>"
}
func (*ConfigPackageName) Severity() cop.Severity { return cop.Error }

func (c *ConfigPackageName) Check(fset *token.FileSet, file *ast.File) []cop.Offense {
	filename := fset.Position(file.Package).Filename
	dir := configDir(filename)
	if dir == "" {
		return nil
	}

	got := file.Name.Name
	switch got {
	case dir:
		return nil
	case dir + "_test":
		// Black-box test packages are a legitimate Go convention.
		if strings.HasSuffix(filename, "_test.go") {
			return nil
		}
	}

	return []cop.Offense{offense(c, fset, file.Name,
		fmt.Sprintf("file in pkg/config/%s/ must declare package %s, got %s", dir, dir, got))}
}
