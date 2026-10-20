package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// updateGo updates each Go module listed in go.work on its own:
//  1. move its dependencies to their newest versions
//  2. record in nix/go-modules.json the local modules it uses and the hash
//     of its external dependencies, or none if it has no external ones
func updateGo() error {
	modules, err := repo.WorkspaceModules()
	if err != nil {
		return err
	}
	entries := repo.GoModules{}
	for _, module := range modules {
		if err := updateDependencies(module); err != nil {
			return err
		}
		local, err := repo.LocalReplacements(module)
		if err != nil {
			return err
		}
		entries[module] = repo.GoModule{Local: local}
		hash, err := dependencyHash(module, entries)
		if err != nil {
			return err
		}
		entries[module] = repo.GoModule{Hash: hash, Local: local}
	}
	return repo.SaveGoModules(entries)
}

func updateDependencies(module string) error {
	inModule := sh.Cmd{Dir: module, Env: []string{"GOWORK=off", "CGO_ENABLED=0"}}
	if err := inModule.Run("go", "get", "-u", "./..."); err != nil {
		return err
	}
	return inModule.Run("go", "mod", "tidy")
}

// dependencyHash is the hash of a module's downloaded external
// dependencies. Nix reports it when it builds them with an empty hash. A
// module without external dependencies has none.
func dependencyHash(module string, entries repo.GoModules) (*string, error) {
	if !hasExternalDependencies(module) {
		return nil, nil
	}
	empty := ""
	entry := entries[module]
	entry.Hash = &empty
	entries[module] = entry
	if err := repo.SaveGoModules(entries); err != nil {
		return nil, err
	}
	hash, err := hashReportedByNix(".#go-deps-" + strings.ReplaceAll(module, "/", "-"))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", module, err)
	}
	return &hash, nil
}

// hasExternalDependencies is true if go.sum lists anything. Local modules,
// used through replace directives, do not appear there.
func hasExternalDependencies(module string) bool {
	sum, err := os.ReadFile(filepath.Join(module, "go.sum"))
	return err == nil && len(strings.TrimSpace(string(sum))) > 0
}
