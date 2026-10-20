package repo

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// WorkspaceModules lists the module directories in go.work.
func WorkspaceModules() ([]string, error) {
	out, err := sh.Output("go", "work", "edit", "-json")
	if err != nil {
		return nil, err
	}
	var work struct{ Use []struct{ DiskPath string } }
	if err := json.Unmarshal([]byte(out), &work); err != nil {
		return nil, err
	}
	var modules []string
	for _, u := range work.Use {
		modules = append(modules, filepath.Clean(u.DiskPath))
	}
	return modules, nil
}

// LocalReplacements lists the module directories that module's go.mod
// replaces with a local path, relative to the repository root.
func LocalReplacements(module string) ([]string, error) {
	out, err := sh.Cmd{Dir: module, Env: []string{"GOWORK=off"}}.Output("go", "mod", "edit", "-json")
	if err != nil {
		return nil, err
	}
	var goMod struct {
		Replace []struct{ New struct{ Path string } }
	}
	if err := json.Unmarshal([]byte(out), &goMod); err != nil {
		return nil, err
	}
	var dirs []string
	for _, r := range goMod.Replace {
		if strings.HasPrefix(r.New.Path, ".") {
			dirs = append(dirs, filepath.Join(module, r.New.Path))
		}
	}
	return dirs, nil
}
