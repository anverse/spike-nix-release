package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// dist builds a component's release with Nix into dist/<component>, next to
// its release notes. Everything a release publishes comes from there.
// A library's release is its notes only.
func dist(component, tag string) error {
	project, err := repo.LoadProject()
	if err != nil {
		return err
	}
	c, err := project.Component(component)
	if err != nil {
		return err
	}
	dir := filepath.Join("dist", component)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if c.Kind == "library" {
		err = os.MkdirAll(dir, 0o755)
	} else {
		err = copyBuild(component, dir)
	}
	if err != nil {
		return err
	}

	text, err := notes(component, tag)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "notes.md"), []byte(text), 0o644)
}

// copyBuild builds the component's release and copies it into dir.
func copyBuild(component, dir string) error {
	built, err := nixOutput("build", ".#"+component+"-release", "--no-link", "--print-out-paths")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if err := sh.Run("cp", "-RL", built, dir); err != nil {
		return err
	}
	return sh.Run("chmod", "-R", "u+w", dir)
}

// verify checks that a release tag may be published: it is on main, the
// checkout is that tag, and versions.json names the tag's version.
func verify(tag string) error {
	r, err := releaseOf(tag)
	if err != nil {
		return err
	}
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	if got := versions[r.component]; got != r.version {
		return fmt.Errorf("versions.json has %s %s, the tag says %s", r.component, got, r.version)
	}
	head, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	tagged, err := gitOutput("rev-list", "-n1", tag)
	if err != nil {
		return err
	}
	if head != tagged {
		return fmt.Errorf("the checkout is not %s", tag)
	}
	if err := git("fetch", "-q", "origin", "main"); err != nil {
		return err
	}
	if err := git("merge-base", "--is-ancestor", tag, "origin/main"); err != nil {
		return fmt.Errorf("%s is not on main", tag)
	}
	return nil
}

// login logs regctl and helm in to ghcr.io with the workflow's token.
func login() error {
	user, token := os.Getenv("GITHUB_ACTOR"), os.Getenv("GITHUB_TOKEN")
	if user == "" || token == "" {
		return fmt.Errorf("GITHUB_ACTOR and GITHUB_TOKEN must be set")
	}
	withToken := sh.Cmd{Stdin: token}
	if err := withToken.Run("regctl", "registry", "login", "ghcr.io", "-u", user, "--pass-stdin"); err != nil {
		return err
	}
	return withToken.Run("helm", "registry", "login", "ghcr.io", "-u", user, "--password-stdin")
}

// publish publishes the release in dist/<component>, which the workflow's
// build job made with `release dist`:
//  1. check that dist/<component> holds the release and its notes are clean
//  2. publish it: a CLI to GitHub, then Homebrew and Scoop; a service to the
//     registry, then GitHub; a library to GitHub
//
// Every step is safe to repeat, so a failed run can be run again.
func publish(tag string) error {
	r, err := releaseOf(tag)
	if err != nil {
		return err
	}
	if err := r.checkDist(); err != nil {
		return err
	}
	switch r.kind {
	case "cli":
		return r.publishCLI()
	case "service":
		return r.publishService()
	case "library":
		return r.publishOnGitHub(nil)
	}
	return fmt.Errorf("component %s has unknown kind %q", r.component, r.kind)
}

// release is one component release being published.
type release struct {
	project                       repo.Project
	component, kind, version, tag string
	dir                           string // dist/<component>
}

func releaseOf(tag string) (release, error) {
	component, version, err := repo.ParseTag(tag)
	if err != nil {
		return release{}, err
	}
	if !semver.MatchString(version) {
		return release{}, fmt.Errorf("version %q is not X.Y.Z", version)
	}
	project, err := repo.LoadProject()
	if err != nil {
		return release{}, err
	}
	c, err := project.Component(component)
	if err != nil {
		return release{}, err
	}
	return release{
		project:   project,
		component: component,
		kind:      c.Kind,
		version:   version,
		tag:       tag,
		dir:       filepath.Join("dist", component),
	}, nil
}

// checkDist fails unless dist/<component> has release notes without a
// signature in them.
func (r release) checkDist() error {
	text, err := os.ReadFile(filepath.Join(r.dir, "notes.md"))
	if err != nil {
		return fmt.Errorf("no release in %s; run `release dist` first: %w", r.dir, err)
	}
	if strings.Contains(string(text), "-----BEGIN ") {
		return fmt.Errorf("%s/notes.md contains a signature block", r.dir)
	}
	return nil
}

// publishCLI publishes the GitHub release with the archives first, then the
// Homebrew formula and the Scoop manifest, which point at those archives.
func (r release) publishCLI() error {
	archives, err := filepath.Glob(filepath.Join(r.dir, "archives", "*"))
	if err != nil || len(archives) == 0 {
		return fmt.Errorf("no archives in %s/archives", r.dir)
	}
	if err := r.publishOnGitHub(archives); err != nil {
		return err
	}
	version := " version v" + r.version
	formula := manifest{repo: r.project.Tap, dir: "Formula", file: r.component + ".rb",
		message: "Brew formula update for " + r.component + version, tokenVariable: "TAP_TOKEN"}
	scoop := manifest{repo: r.project.Bucket, dir: "bucket", file: r.component + ".json",
		message: "Scoop update for " + r.component + version, tokenVariable: "BUCKET_TOKEN"}
	if err := r.push(formula); err != nil {
		return err
	}
	return r.push(scoop)
}

// publishService pushes the image and the chart, then publishes the GitHub
// release, whose notes name the image digest.
func (r release) publishService() error {
	image := fmt.Sprintf("%s/%s:%s", r.project.Registry, r.component, r.version)
	chart := filepath.Join(r.dir, "chart", r.component+"-"+r.version+".tgz")

	if err := regctl("image", "copy", "ocidir://"+filepath.Join(r.dir, "image")+":"+r.version, image); err != nil {
		return err
	}
	if err := helm("push", chart, "oci://"+r.project.Registry+"/charts"); err != nil {
		return err
	}
	if err := r.addArtifactsToNotes(image); err != nil {
		return err
	}
	return r.publishOnGitHub([]string{chart})
}

// addArtifactsToNotes appends the image and chart to the notes, once.
func (r release) addArtifactsToNotes(image string) error {
	path := filepath.Join(r.dir, "notes.md")
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(text), "## Artifacts") {
		return nil
	}
	digest, err := imageDigest(filepath.Join(r.dir, "image"))
	if err != nil {
		return err
	}
	artifacts := fmt.Sprintf("\n## Artifacts\n\n"+
		"- Image: `%s` (`%s`), linux/amd64 and linux/arm64\n"+
		"- Chart: `oci://%s/charts/%s`, version %s\n",
		image, digest, r.project.Registry, r.component, r.version)
	return os.WriteFile(path, append(text, artifacts...), 0o644)
}

// publishOnGitHub publishes the release for the tag:
//  1. create it as a draft, unless it exists
//  2. upload the files, replacing any from an earlier attempt
//  3. publish it with the notes; only the component named "latest" in
//     project.json becomes the repository's latest release
func (r release) publishOnGitHub(files []string) error {
	notesFile := filepath.Join(r.dir, "notes.md")
	if _, err := sh.Output("gh", "release", "view", r.tag); err != nil {
		if err := gh("release", "create", r.tag, "--verify-tag", "--draft",
			"--title", r.component+" v"+r.version, "--notes-file", notesFile); err != nil {
			return err
		}
	}
	if len(files) > 0 {
		if err := gh(append([]string{"release", "upload", r.tag, "--clobber"}, files...)...); err != nil {
			return err
		}
	}
	return gh("release", "edit", r.tag, "--draft=false", "--notes-file", notesFile,
		fmt.Sprintf("--latest=%t", r.component == r.project.Latest))
}

// manifest is a rendered Homebrew formula or Scoop manifest and where it
// goes.
type manifest struct {
	repo, dir, file, message string
	// tokenVariable names the environment variable with a GitHub App token
	// for the repository (CI); without it, gh's own login is used.
	tokenVariable string
}

// push commits the manifest into its repository, unless it is current:
// clone, copy, commit, push.
func (r release) push(m manifest) error {
	work, err := os.MkdirTemp("", "manifest-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(work); err != nil {
			fmt.Fprintln(os.Stderr, "remove", work+":", err)
		}
	}()

	var run sh.Cmd
	if token := os.Getenv(m.tokenVariable); token != "" {
		run.Env = []string{"GH_TOKEN=" + token}
	}
	if err := run.Run("gh", "repo", "clone", m.repo, work, "--", "--depth", "1", "-q"); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(r.dir, "manifests", m.file), filepath.Join(work, m.dir, m.file)); err != nil {
		return err
	}
	changes, err := gitOutput("-C", work, "status", "--porcelain")
	if err != nil {
		return err
	}
	if changes == "" {
		fmt.Fprintf(os.Stderr, "%s is already current in %s\n", m.file, m.repo)
		return nil
	}
	run.Dir = work
	if err := run.Run("git", "add", "-A"); err != nil {
		return err
	}
	if err := run.Run("git", "commit", "-q", "-m", m.message); err != nil {
		return err
	}
	return run.Run("git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "-q")
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

// imageDigest reads the digest of the image from an image layout.
func imageDigest(layout string) (string, error) {
	var index struct {
		Manifests []struct{ Digest string }
	}
	b, err := os.ReadFile(filepath.Join(layout, "index.json"))
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(b, &index); err != nil || len(index.Manifests) != 1 {
		return "", fmt.Errorf("%s/index.json: want one image", layout)
	}
	return index.Manifests[0].Digest, nil
}
