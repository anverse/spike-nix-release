// Package project reads the repository's release configuration
// (project.json) and component versions (versions.json), and runs the
// command-line tools the release and development tools depend on.
package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Project is project.json: where releases go and what each component is.
type Project struct {
	Owner      string               `json:"owner"`
	Repo       string               `json:"repo"`
	Registry   string               `json:"registry"`
	Tap        string               `json:"tap"`
	Bucket     string               `json:"bucket"`
	Latest     string               `json:"latest"`
	Components map[string]Component `json:"components"`
}

// Component is one releasable part of the repository.
type Component struct {
	// Kind is "cli" (archives, Homebrew, Scoop) or "service" (image, chart).
	Kind string `json:"kind"`
	// Paths belong to the component: its release notes list the commits
	// that touched them.
	Paths []string `json:"paths"`
}

// Load reads project.json from dir.
func Load(dir string) (Project, error) {
	var p Project
	err := readJSON(dir+"/project.json", &p)
	return p, err
}

// Component returns the named component or an error if it is unknown.
func (p Project) Component(name string) (Component, error) {
	c, ok := p.Components[name]
	if !ok {
		return Component{}, fmt.Errorf("unknown component %q", name)
	}
	return c, nil
}

// LoadVersions reads versions.json from dir: component name to version.
func LoadVersions(dir string) (map[string]string, error) {
	v := map[string]string{}
	err := readJSON(dir+"/versions.json", &v)
	return v, err
}

// SaveVersions writes versions.json to dir.
func SaveVersions(dir string, v map[string]string) error {
	return writeJSON(dir+"/versions.json", v)
}

// Tag is the git tag of a component release, e.g. spikectl/v0.1.0.
func Tag(component, version string) string {
	return component + "/v" + version
}

// ParseTag splits a release tag into component and version.
func ParseTag(tag string) (component, version string, err error) {
	component, v, ok := strings.Cut(tag, "/v")
	if !ok || component == "" || v == "" {
		return "", "", fmt.Errorf("tag %q is not <component>/v<version>", tag)
	}
	return component, v, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Run runs a command, shows it, and passes its output through.
func Run(name string, args ...string) error {
	fmt.Fprintln(os.Stderr, "+", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// RunWith runs a command like Run, with extra environment variables and,
// if stdin is not empty, that text as its input.
func RunWith(env []string, stdin, name string, args ...string) error {
	fmt.Fprintln(os.Stderr, "+", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	return cmd.Run()
}

// Output runs a command and returns its standard output, trimmed.
func Output(name string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
