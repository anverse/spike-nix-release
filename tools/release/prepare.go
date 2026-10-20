package main

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
)

// prepare opens the pull request that releases a component:
//  1. check that the checkout is the current main, clean, and the version new
//  2. on the branch release/<component>-v<version>, write the version into
//     versions.json
//  3. build the release, so a broken build never reaches review
//  4. commit, push the branch, open the pull request, and return to main
//
// After review and merge, `release tag` tags the merged commit.
func prepare(component, version string) error {
	tag := repo.Tag(component, version)
	branch := "release/" + component + "-v" + version

	if err := checkOnCurrentMain(); err != nil {
		return err
	}
	if err := checkNewVersion(component, version, tag); err != nil {
		return err
	}
	if err := git("switch", "-q", "-c", branch); err != nil {
		return err
	}
	if err := setVersion(component, version); err != nil {
		return err
	}
	if err := nix("build", ".#"+component+"-release", "--no-link"); err != nil {
		return errors.Join(fmt.Errorf("the release does not build: %w", err), undoBranch(branch))
	}
	return commitAndOpenPullRequest(component, version, branch)
}

// undoBranch leaves the release branch: restore versions.json, return to
// main, delete the branch.
func undoBranch(branch string) error {
	return errors.Join(
		git("checkout", "versions.json"),
		git("switch", "-q", "main"),
		git("branch", "-q", "-D", branch),
	)
}

var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// checkOnCurrentMain fails unless the checkout is main, clean, and the
// same commit as origin/main.
func checkOnCurrentMain() error {
	if branch, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD"); err != nil || branch != "main" {
		return fmt.Errorf("check out main first")
	}
	if changes, err := gitOutput("status", "--porcelain"); err != nil || changes != "" {
		return fmt.Errorf("the working tree has uncommitted changes")
	}
	if err := git("fetch", "-q", "origin", "main"); err != nil {
		return err
	}
	local, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	remote, err := gitOutput("rev-parse", "origin/main")
	if err != nil {
		return err
	}
	if local != remote {
		return fmt.Errorf("main is not origin/main; pull or push first")
	}
	return nil
}

func checkNewVersion(component, version, tag string) error {
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

func commitAndOpenPullRequest(component, version, branch string) error {
	title := "release " + component + " v" + version
	if err := git("commit", "-q", "-m", title, "versions.json"); err != nil {
		return err
	}
	if err := git("push", "-q", "-u", "origin", branch); err != nil {
		return err
	}
	body := fmt.Sprintf("Sets %s to %s. After merge: `nix run .#release -- tag %s \"<summary>\"`.", component, version, component)
	if err := gh("pr", "create", "--base", "main", "--head", branch, "--title", title, "--body", body); err != nil {
		return err
	}
	return git("switch", "-q", "main")
}
