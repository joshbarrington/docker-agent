// Package main runs project-specific linting cops using rubocop-go.
//
// Usage: go run ./lint [path...]
package main

import (
	"fmt"
	"os"

	"github.com/dgageot/rubocop-go/config"
	"github.com/dgageot/rubocop-go/cop"
	"github.com/dgageot/rubocop-go/runner"
)

// cops is the registry of project-specific cops, in declaration order.
// To add a cop: implement cop.Cop and append it here.
var cops = []cop.Cop{
	&ConfigVersionImport{},
	&ConfigPackageName{},
	&ConfigVersionConstant{},
	&LatestImportsPredecessor{},
}

func main() {
	for _, c := range cops {
		cop.Register(c)
	}
	fmt.Printf("Inspecting Go files with %d cop(s)\n", len(cops))

	paths := os.Args[1:]
	if len(paths) == 0 {
		paths = []string{"."}
	}

	r := runner.New(cop.All(), config.DefaultConfig(), os.Stdout)
	offenseCount, err := r.Run(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if offenseCount > 0 {
		os.Exit(1)
	}
}
