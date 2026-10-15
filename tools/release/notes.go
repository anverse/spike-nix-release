package main

import (
	"fmt"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

// notes writes a component's release notes: the tag's message, then every
// commit since the component's previous release that touched the
// component's paths. Release commits are left out. Without a tag, the
// notes cover the unreleased commits up to HEAD.
func notes(component, tag string) (string, error) {
	p, err := project.Load(".")
	if err != nil {
		return "", err
	}
	c, err := p.Component(component)
	if err != nil {
		return "", err
	}

	to, summary := "HEAD", "Unreleased changes."
	if tag != "" {
		to = tag
		// Subject and body only: a signed tag's signature stays out.
		summary, err = project.Output("git", "tag", "-l", "--format=%(contents:subject)%0a%0a%(contents:body)", tag)
		if err != nil {
			return "", err
		}
	}

	// The component's previous release; none before its first one.
	commits := to
	if prev, err := project.Output("git", "describe", "--tags", "--abbrev=0", "--match", component+"/v*", to+"^"); err == nil {
		commits = prev + ".." + to
	}
	log, err := project.Output("git", append([]string{"log", "--reverse", "--format=%h %s", commits, "--"}, c.Paths...)...)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n## Changes\n\n", strings.TrimSpace(summary))
	for _, line := range strings.Split(log, "\n") {
		hash, subject, ok := strings.Cut(line, " ")
		if !ok || strings.HasPrefix(subject, "release ") {
			continue
		}
		fmt.Fprintf(&b, "- %s (%s)\n", subject, hash)
	}
	return b.String(), nil
}
