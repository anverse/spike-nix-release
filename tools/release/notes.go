package main

import (
	"fmt"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
)

// notes writes a component's release notes:
//  1. the summary: the tag's message without its signature, or
//     "Unreleased changes." without a tag
//  2. every commit since the component's previous release that touched the
//     component's paths, oldest first
//
// Release commits change only versions.json, which belongs to no
// component, so they never appear.
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
// modules it uses (from nix/go-modules.json, e.g. api), and the extra paths
// in project.json (e.g. its chart).
func componentPaths(component string) ([]string, error) {
	project, err := repo.LoadProject()
	if err != nil {
		return nil, err
	}
	c, err := project.Component(component)
	if err != nil {
		return nil, err
	}
	modules, err := repo.LoadGoModules()
	if err != nil {
		return nil, err
	}
	paths := append([]string{c.Module}, modules[c.Module].Local...)
	return append(paths, c.Paths...), nil
}

func summaryOf(tag string) (string, error) {
	if tag == "" {
		return "Unreleased changes.", nil
	}
	raw, err := gitOutput("cat-file", "tag", tag)
	if err != nil {
		return "", fmt.Errorf("%s is not an annotated tag: %w", tag, err)
	}
	return tagMessage(raw), nil
}

// tagMessage is the message of an annotated tag (`git cat-file tag`), without
// the header before it and without a signature (PGP, SSH or X.509) after it.
func tagMessage(raw string) string {
	_, message, _ := strings.Cut(raw, "\n\n")
	if i := strings.Index(message, "-----BEGIN "); i >= 0 {
		message = message[:i]
	}
	return strings.TrimSpace(message)
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
		if hash, subject, ok := strings.Cut(line, " "); ok {
			result = append(result, commit{hash, subject})
		}
	}
	return result, nil
}
