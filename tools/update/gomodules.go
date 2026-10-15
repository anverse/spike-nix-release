package main

import (
	"encoding/json"
	"fmt"
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

// dependencyHash is the hash of a module's downloaded dependencies, local
// modules included. Nix reports it when it builds them with an empty hash.
func dependencyHash(module string, hashes repo.GoModules) (*string, error) {
	requires, err := hasRequirements(module)
	if err != nil || !requires {
		return nil, err
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

// hasRequirements is true if the module's go.mod requires any module, local
// ones (through replace directives) included.
func hasRequirements(module string) (bool, error) {
	out, err := sh.Cmd{Dir: module, Env: []string{"GOWORK=off"}}.Output("go", "mod", "edit", "-json")
	if err != nil {
		return false, err
	}
	var goMod struct{ Require []struct{ Path string } }
	if err := json.Unmarshal([]byte(out), &goMod); err != nil {
		return false, err
	}
	return len(goMod.Require) > 0, nil
}
