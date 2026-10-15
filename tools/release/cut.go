package main

import (
	"fmt"
	"regexp"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// cut releases a component: it writes the version into versions.json,
// checks that the release builds, commits, creates an annotated tag whose
// message starts the release notes, and pushes commit and tag together.
// The pushed tag starts the release workflow.
func cut(component, version, message string, push bool) error {
	p, err := project.Load(".")
	if err != nil {
		return err
	}
	if _, err := p.Component(component); err != nil {
		return err
	}
	if !semver.MatchString(version) {
		return fmt.Errorf("version %q is not X.Y.Z", version)
	}
	tag := project.Tag(component, version)
	if status, err := project.Output("git", "status", "--porcelain"); err != nil || status != "" {
		return fmt.Errorf("working tree has uncommitted changes")
	}
	if _, err := project.Output("git", "rev-parse", "-q", "--verify", "refs/tags/"+tag); err == nil {
		return fmt.Errorf("tag %s exists", tag)
	}

	versions, err := project.LoadVersions(".")
	if err != nil {
		return err
	}
	versions[component] = version
	if err := project.SaveVersions(".", versions); err != nil {
		return err
	}
	if err := project.Run("nix", "build", ".#"+component+"-release", "--no-link"); err != nil {
		project.Run("git", "checkout", "versions.json")
		return fmt.Errorf("the release does not build: %w", err)
	}

	steps := [][]string{
		{"git", "commit", "-q", "-m", "release " + component + " v" + version, "versions.json"},
		{"git", "tag", "-a", tag, "-m", message},
	}
	if push {
		steps = append(steps, []string{"git", "push", "--atomic", "origin", "HEAD", tag})
	}
	for _, s := range steps {
		if err := project.Run(s[0], s[1:]...); err != nil {
			return err
		}
	}
	return nil
}
