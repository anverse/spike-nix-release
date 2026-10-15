package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// dist builds a component's release with Nix into dist/<component>, next to
// its release notes. Everything a release publishes comes from there.
func dist(component, tag string) error {
	built, err := nixOutput("build", ".#"+component+"-release", "--no-link", "--print-out-paths")
	if err != nil {
		return err
	}
	dir := filepath.Join("dist", component)
	os.RemoveAll(dir)
	os.MkdirAll("dist", 0o755)
	if err := sh.Run("cp", "-RL", built, dir); err != nil {
		return err
	}
	if err := sh.Run("chmod", "-R", "u+w", dir); err != nil {
		return err
	}

	text, err := notes(component, tag)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "notes.md"), []byte(text), 0o644)
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

// publish publishes the release of a pushed tag, in the release workflow:
//  1. check that the checkout is that tag and versions.json agrees
//  2. build the release into dist/, with its notes
//  3. publish it: a CLI to GitHub, Homebrew and Scoop; a service to the
//     registry and GitHub
func publish(tag string) error {
	r, err := releaseOf(tag)
	if err != nil {
		return err
	}
	if err := r.checkCheckout(); err != nil {
		return err
	}
	// actions/checkout turns annotated tags into lightweight ones; fetch the
	// real tag, whose message starts the release notes.
	if err := git("fetch", "-q", "--force", "origin", "refs/tags/"+tag+":refs/tags/"+tag); err != nil {
		return err
	}
	if err := dist(r.component, tag); err != nil {
		return err
	}
	switch r.kind {
	case "cli":
		return r.publishCLI()
	case "service":
		return r.publishService()
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

func (r release) checkCheckout() error {
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	if versions[r.component] != r.version {
		return fmt.Errorf("versions.json has %s %s, the tag says %s", r.component, versions[r.component], r.version)
	}
	head, _ := gitOutput("rev-parse", "HEAD")
	tagged, _ := gitOutput("rev-list", "-n1", r.tag)
	if head != tagged {
		return fmt.Errorf("the checkout is not %s", r.tag)
	}
	return nil
}

// publishCLI creates the GitHub release with the archives, then updates the
// Homebrew tap and the Scoop bucket, which point at those archives.
func (r release) publishCLI() error {
	archives, _ := filepath.Glob(filepath.Join(r.dir, "archives", "*"))
	if err := r.githubRelease(archives); err != nil {
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

// publishService pushes the image and the chart, then creates the GitHub
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
	return r.githubRelease([]string{chart})
}

func (r release) addArtifactsToNotes(image string) error {
	digest, err := imageDigest(filepath.Join(r.dir, "image"))
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(r.dir, "notes.md"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n## Artifacts\n\n"+
		"- Image: `%s` (`%s`), linux/amd64 and linux/arm64\n"+
		"- Chart: `oci://%s/charts/%s`, version %s\n",
		image, digest, r.project.Registry, r.component, r.version)
	return err
}

// githubRelease creates the release for the tag. Only the component named
// "latest" in project.json becomes the repository's latest release.
func (r release) githubRelease(files []string) error {
	args := []string{"release", "create", r.tag, "--verify-tag",
		"--title", r.component + " v" + r.version,
		"--notes-file", filepath.Join(r.dir, "notes.md"),
		fmt.Sprintf("--latest=%t", r.component == r.project.Latest)}
	return gh(append(args, files...)...)
}

// manifest is a rendered Homebrew formula or Scoop manifest and where it
// goes.
type manifest struct {
	repo, dir, file, message string
	// tokenVariable names the environment variable with a GitHub App token
	// for the repository (CI); without it, gh's own login is used.
	tokenVariable string
}

// push commits the manifest into its repository: clone, copy, commit, push.
func (r release) push(m manifest) error {
	work, err := os.MkdirTemp("", "manifest-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

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
	if changes, _ := gitOutput("-C", work, "status", "--porcelain"); changes == "" {
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
