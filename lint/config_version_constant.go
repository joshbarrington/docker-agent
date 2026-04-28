package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/dgageot/rubocop-go/cop"
)

// ConfigVersionConstant enforces that a file under pkg/config/vN/ declaring a
// `const Version = "<value>"` uses "<N>" as its value.
//
// This guards against the common mistake of bumping the directory name
// without bumping the constant (or vice versa) when freezing the
// work-in-progress config and creating a new "latest". A mismatch would
// silently break the parser dispatch in pkg/config/versions.go, which
// registers parsers keyed by `Version`.
//
// Files under pkg/config/latest/ are intentionally exempt: their `Version`
// is the next, work-in-progress value (one greater than the highest vN).
type ConfigVersionConstant struct{}

func (*ConfigVersionConstant) Name() string { return "Lint/ConfigVersionConstant" }
func (*ConfigVersionConstant) Description() string {
	return "Version constant in pkg/config/vN/ must equal \"N\""
}
func (*ConfigVersionConstant) Severity() cop.Severity { return cop.Error }

func (c *ConfigVersionConstant) Check(fset *token.FileSet, file *ast.File) []cop.Offense {
	dirVersion, ok := versionFromDir(configDir(fset.Position(file.Package).Filename))
	if !ok {
		return nil
	}
	expected := strconv.Itoa(dirVersion)

	var offenses []cop.Offense
	for _, lit := range versionStringLiterals(file) {
		got, err := strconv.Unquote(lit.Value)
		if err != nil || got == expected {
			continue
		}
		offenses = append(offenses, offense(c, fset, lit,
			fmt.Sprintf("Version in pkg/config/v%s/ must be %q, got %q", expected, expected, got)))
	}
	return offenses
}

// versionStringLiterals returns the value literal of every top-level
// `const Version = "<string>"` declaration in file.
func versionStringLiterals(file *ast.File) []*ast.BasicLit {
	var lits []*ast.BasicLit
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name.Name != "Version" || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				lits = append(lits, lit)
			}
		}
	}
	return lits
}
