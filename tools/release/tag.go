package main

import (
	"fmt"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
)

// tagRelease tags a merged release and pushes the tag, which starts the
// release workflow:
//  1. check that the checkout is the current main, clean
//  2. read the component's version from versions.json; its tag must be new
//  3. create an annotated tag with the summary on that commit, and push it
func tagRelease(component, summary string) error {
	if err := checkOnCurrentMain(); err != nil {
		return err
	}
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	version, ok := versions[component]
	if !ok {
		return fmt.Errorf("unknown component %q", component)
	}
	tag := repo.Tag(component, version)
	if err := checkNewVersion(component, version, tag); err != nil {
		return fmt.Errorf("%w; prepare a new version first", err)
	}
	if err := git("tag", "-a", tag, "-m", summary); err != nil {
		return err
	}
	return git("push", "-q", "origin", tag)
}
