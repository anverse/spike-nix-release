package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// notes writes a component's release notes:
//  1. the summary: the tag's message, or "Unreleased changes." without a tag
//  2. every commit since the component's previous release that touched the
//     component's paths, oldest first, without release commits
func notes(component, tag string) (string, error) {
	paths, err := componentPaths(component)
	if err != nil {
		return "", err
	}
	summary, err := summaryOf(tag)
	if err != nil {
		return "", err
	}
	commits, err := commitsSincePreviousRelease(component, tag, paths)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n## Changes\n\n", summary)
	for _, c := range commits {
		fmt.Fprintf(&b, "- %s (%s)\n", c.subject, c.hash)
	}
	return b.String(), nil
}

// componentPaths is what belongs to a component: its Go module, the local
// modules it uses through replace directives (e.g. api), and the extra
// paths in project.json (e.g. its chart).
func componentPaths(component string) ([]string, error) {
	project, err := repo.LoadProject()
	if err != nil {
		return nil, err
	}
	c, err := project.Component(component)
	if err != nil {
		return nil, err
	}
	local, err := localModules(c.Module)
	if err != nil {
		return nil, err
	}
	paths := append([]string{c.Module}, local...)
	return append(paths, c.Paths...), nil
}

// localModules lists the directories of the modules that module replaces
// with a local path.
func localModules(module string) ([]string, error) {
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

func summaryOf(tag string) (string, error) {
	if tag == "" {
		return "Unreleased changes.", nil
	}
	// Subject and body only: a signed tag's signature stays out.
	return gitOutput("tag", "-l", "--format=%(contents:subject)%0a%0a%(contents:body)", tag)
}

type commit struct{ hash, subject string }

func commitsSincePreviousRelease(component, tag string, paths []string) ([]commit, error) {
	to := tag
	if to == "" {
		to = "HEAD"
	}
	// From the component's previous release, or from the start for its first.
	commits := to
	if previous, err := gitOutput("describe", "--tags", "--abbrev=0", "--match", component+"/v*", to+"^"); err == nil {
		commits = previous + ".." + to
	}
	log, err := gitOutput(append([]string{"log", "--reverse", "--format=%h %s", commits, "--"}, paths...)...)
	if err != nil {
		return nil, err
	}

	var result []commit
	for _, line := range strings.Split(log, "\n") {
		hash, subject, ok := strings.Cut(line, " ")
		if ok && !strings.HasPrefix(subject, "release ") {
			result = append(result, commit{hash, subject})
		}
	}
	return result, nil
}
