package main

import (
	"fmt"
	"regexp"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
)

// cut releases a component:
//  1. check that the version is new and the working tree is clean
//  2. write the version into versions.json
//  3. build the release, so a tag never points at a broken build
//  4. commit, tag with the summary, and push both together
func cut(component, version, summary string, push bool) error {
	tag := repo.Tag(component, version)

	if err := checkCanRelease(component, version, tag); err != nil {
		return err
	}
	if err := setVersion(component, version); err != nil {
		return err
	}
	if err := nix("build", ".#"+component+"-release", "--no-link"); err != nil {
		git("checkout", "versions.json") // undo step 2
		return fmt.Errorf("the release does not build: %w", err)
	}
	return commitTagAndPush(component, version, tag, summary, push)
}

var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func checkCanRelease(component, version, tag string) error {
	project, err := repo.LoadProject()
	if err != nil {
		return err
	}
	if _, err := project.Component(component); err != nil {
		return err
	}
	if !semver.MatchString(version) {
		return fmt.Errorf("version %q is not X.Y.Z", version)
	}
	if changes, err := gitOutput("status", "--porcelain"); err != nil || changes != "" {
		return fmt.Errorf("the working tree has uncommitted changes")
	}
	if _, err := gitOutput("rev-parse", "-q", "--verify", "refs/tags/"+tag); err == nil {
		return fmt.Errorf("tag %s exists", tag)
	}
	return nil
}

func setVersion(component, version string) error {
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	versions[component] = version
	return repo.SaveVersions(versions)
}

func commitTagAndPush(component, version, tag, summary string, push bool) error {
	if err := git("commit", "-q", "-m", "release "+component+" v"+version, "versions.json"); err != nil {
		return err
	}
	if err := git("tag", "-a", tag, "-m", summary); err != nil {
		return err
	}
	if !push {
		return nil
	}
	return git("push", "--atomic", "origin", "HEAD", tag)
}
