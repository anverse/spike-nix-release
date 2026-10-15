package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

// dist builds a component's release with Nix and copies it into
// dist/<component>, next to its release notes. Everything a release
// publishes comes from here.
func dist(component, tag string) error {
	built, err := project.Output("nix", "build", ".#"+component+"-release", "--no-link", "--print-out-paths")
	if err != nil {
		return err
	}
	dir := filepath.Join("dist", component)
	os.RemoveAll(dir)
	os.MkdirAll("dist", 0o755)
	if err := project.Run("cp", "-RL", built, dir); err != nil {
		return err
	}
	if err := project.Run("chmod", "-R", "u+w", dir); err != nil {
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
	if err := project.RunWith(nil, token, "regctl", "registry", "login", "ghcr.io", "-u", user, "--pass-stdin"); err != nil {
		return err
	}
	return project.RunWith(nil, token, "helm", "registry", "login", "ghcr.io", "-u", user, "--password-stdin")
}

// publish publishes the release of a pushed tag. It runs in the release
// workflow, on a checkout of that tag.
func publish(tag string) error {
	component, version, err := project.ParseTag(tag)
	if err != nil {
		return err
	}
	p, err := project.Load(".")
	if err != nil {
		return err
	}
	c, err := p.Component(component)
	if err != nil {
		return err
	}
	versions, err := project.LoadVersions(".")
	if err != nil {
		return err
	}
	if versions[component] != version {
		return fmt.Errorf("versions.json has %s %s, the tag says %s", component, versions[component], version)
	}
	head, _ := project.Output("git", "rev-parse", "HEAD")
	tagged, _ := project.Output("git", "rev-list", "-n1", tag)
	if head != tagged {
		return fmt.Errorf("HEAD is not %s", tag)
	}

	// actions/checkout turns annotated tags into lightweight ones; fetch
	// the real tag, whose message starts the release notes.
	if err := project.Run("git", "fetch", "-q", "--force", "origin", "refs/tags/"+tag+":refs/tags/"+tag); err != nil {
		return err
	}
	if err := dist(component, tag); err != nil {
		return err
	}

	r := release{project: p, component: component, version: version, tag: tag, dir: filepath.Join("dist", component)}
	switch c.Kind {
	case "cli":
		return r.publishCLI()
	case "service":
		return r.publishService()
	}
	return fmt.Errorf("component %s has unknown kind %q", component, c.Kind)
}

type release struct {
	project                 project.Project
	component, version, tag string
	dir                     string
}

// publishCLI creates the GitHub release with the archives, then updates the
// Homebrew tap and the Scoop bucket, which point at those archives.
func (r release) publishCLI() error {
	archives, _ := filepath.Glob(filepath.Join(r.dir, "archives", "*"))
	if err := r.githubRelease(archives); err != nil {
		return err
	}
	v := " version v" + r.version
	if err := r.pushManifest(r.project.Tap, "Formula", r.component+".rb",
		"Brew formula update for "+r.component+v, "TAP_TOKEN"); err != nil {
		return err
	}
	return r.pushManifest(r.project.Bucket, "bucket", r.component+".json",
		"Scoop update for "+r.component+v, "BUCKET_TOKEN")
}

// publishService pushes the image and the chart, then creates the GitHub
// release, whose notes name the image digest.
func (r release) publishService() error {
	image := fmt.Sprintf("%s/%s:%s", r.project.Registry, r.component, r.version)
	if err := project.Run("regctl", "image", "copy", "ocidir://"+filepath.Join(r.dir, "image")+":"+r.version, image); err != nil {
		return err
	}
	chart := filepath.Join(r.dir, "chart", r.component+"-"+r.version+".tgz")
	if err := project.Run("helm", "push", chart, "oci://"+r.project.Registry+"/charts"); err != nil {
		return err
	}
	digest, err := imageDigest(filepath.Join(r.dir, "image"))
	if err != nil {
		return err
	}
	artifacts := fmt.Sprintf("\n## Artifacts\n\n"+
		"- Image: `%s` (`%s`), linux/amd64 and linux/arm64\n"+
		"- Chart: `oci://%s/charts/%s`, version %s\n", image, digest, r.project.Registry, r.component, r.version)
	f, err := os.OpenFile(filepath.Join(r.dir, "notes.md"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	f.WriteString(artifacts)
	f.Close()
	return r.githubRelease([]string{chart})
}

// githubRelease creates the release for the tag. Only the component named
// "latest" in project.json becomes the repository's latest release.
func (r release) githubRelease(files []string) error {
	latest := fmt.Sprintf("--latest=%t", r.component == r.project.Latest)
	args := append([]string{"release", "create", r.tag, "--verify-tag",
		"--title", r.component + " v" + r.version,
		"--notes-file", filepath.Join(r.dir, "notes.md"), latest}, files...)
	return project.Run("gh", args...)
}

// pushManifest commits a rendered manifest into the tap or bucket
// repository. In CI, tokenEnv names a GitHub App token for that
// repository; locally, gh's own login is used.
func (r release) pushManifest(repo, dir, file, message, tokenEnv string) error {
	var env []string
	if token := os.Getenv(tokenEnv); token != "" {
		env = []string{"GH_TOKEN=" + token}
	}
	work, err := os.MkdirTemp("", "manifest-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	if err := project.RunWith(env, "", "gh", "repo", "clone", repo, work, "--", "--depth", "1", "-q"); err != nil {
		return err
	}
	os.MkdirAll(filepath.Join(work, dir), 0o755)
	manifest, err := os.ReadFile(filepath.Join(r.dir, "manifests", file))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, dir, file), manifest, 0o644); err != nil {
		return err
	}
	if status, _ := project.Output("git", "-C", work, "status", "--porcelain"); status == "" {
		fmt.Fprintf(os.Stderr, "%s is already current in %s\n", file, repo)
		return nil
	}
	for _, s := range [][]string{
		{"git", "-C", work, "add", "-A"},
		{"git", "-C", work, "commit", "-q", "-m", message},
		{"git", "-C", work, "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "-q"},
	} {
		if err := project.RunWith(env, "", s[0], s[1:]...); err != nil {
			return err
		}
	}
	return nil
}

// imageDigest reads the digest of the image index from an image layout.
func imageDigest(layout string) (string, error) {
	var index struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
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
