package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
)

// checkConfig checks that the files describing the repository agree; a
// flake check runs it:
//  1. go.work and nix/go-modules.json list the same modules
//  2. every module's local modules in nix/go-modules.json are the ones its
//     go.mod replaces with a local path
//  3. project.json and versions.json list the same components, each with a
//     module from go.work
//  4. the shared service chart exists, and every CLI has its manifest templates
func checkConfig() error {
	project, err := repo.LoadProject()
	if err != nil {
		return err
	}
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	modules, err := repo.LoadGoModules()
	if err != nil {
		return err
	}
	workspace, err := repo.WorkspaceModules()
	if err != nil {
		return err
	}

	var problems []string
	problems = append(problems, sameSets("go.work", workspace, "nix/go-modules.json", keys(modules))...)
	for module, m := range modules {
		local, err := repo.LocalReplacements(module)
		if err != nil {
			return err
		}
		problems = append(problems, sameSets(module+"/go.mod replace", local, "nix/go-modules.json "+module+" local", m.Local)...)
	}
	problems = append(problems, sameSets("project.json", keys(project.Components), "versions.json", keys(versions))...)
	for name, c := range project.Components {
		if !slices.Contains(workspace, c.Module) {
			problems = append(problems, fmt.Sprintf("component %s: module %q is not in go.work", name, c.Module))
		}
		problems = append(problems, missingFiles(name, c.Kind)...)
	}
	if len(problems) > 0 {
		return fmt.Errorf("the configuration does not agree:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func missingFiles(component, kind string) []string {
	var want string
	switch kind {
	case "service":
		// Every service uses the shared chart (nix/service.nix).
		want = filepath.Join("charts", "service", "Chart.yaml")
	case "cli":
		want = filepath.Join("packaging", "homebrew", component+".rb.tmpl")
	default:
		return nil
	}
	if _, err := os.Stat(want); err != nil {
		return []string{fmt.Sprintf("component %s: %s is missing", component, want)}
	}
	return nil
}

// sameSets describes the differences between two lists of names.
func sameSets(aName string, a []string, bName string, b []string) []string {
	var problems []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			problems = append(problems, fmt.Sprintf("%s has %s, %s does not", aName, x, bName))
		}
	}
	for _, x := range b {
		if !slices.Contains(a, x) {
			problems = append(problems, fmt.Sprintf("%s has %s, %s does not", bName, x, aName))
		}
	}
	return problems
}

func keys[V any](m map[string]V) []string {
	var k []string
	for name := range m {
		k = append(k, name)
	}
	slices.Sort(k)
	return k
}
