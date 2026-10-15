package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// updateGo updates each Go module listed in go.work on its own:
//  1. move its dependencies to their newest versions
//  2. record the hash of its dependencies for the Nix build, or none if it
//     has no external dependencies
func updateGo() error {
	modules, err := workspaceModules()
	if err != nil {
		return err
	}
	hashes := repo.GoModules{}
	for _, module := range modules {
		if err := updateDependencies(module); err != nil {
			return err
		}
		hash, err := dependencyHash(module, hashes)
		if err != nil {
			return err
		}
		hashes[module] = hash
	}
	return repo.SaveGoModules(hashes)
}

// workspaceModules lists the module directories in go.work.
func workspaceModules() ([]string, error) {
	out, err := sh.Output("go", "work", "edit", "-json")
	if err != nil {
		return nil, err
	}
	var work struct {
		Use []struct{ DiskPath string }
	}
	if err := json.Unmarshal([]byte(out), &work); err != nil {
		return nil, err
	}
	var modules []string
	for _, u := range work.Use {
		modules = append(modules, filepath.Clean(u.DiskPath))
	}
	return modules, nil
}

func updateDependencies(module string) error {
	inModule := sh.Cmd{Dir: module, Env: []string{"GOWORK=off", "CGO_ENABLED=0"}}
	if err := inModule.Run("go", "get", "-u", "./..."); err != nil {
		return err
	}
	return inModule.Run("go", "mod", "tidy")
}

// dependencyHash is the hash of a module's downloaded external
// dependencies. Nix reports it when it builds them with an empty hash.
// A module without external dependencies has none.
func dependencyHash(module string, hashes repo.GoModules) (*string, error) {
	if !hasExternalDependencies(module) {
		return nil, nil
	}
	empty := ""
	hashes[module] = &empty
	if err := repo.SaveGoModules(hashes); err != nil {
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
