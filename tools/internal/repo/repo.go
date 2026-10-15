// Package repo reads and writes the repository's configuration files:
//
//	project.json          components, repository, registry, tap, bucket
//	versions.json         the current version of each component
//	pins.json             external artifacts the build downloads
//	nix/go-modules.json   the Go modules and the hash of their dependencies
//
// All paths are relative to the repository root, the tools' working
// directory.
package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Project is project.json.
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
	// Module is the directory of the component's Go module.
	Module string `json:"module"`
	// Paths outside the module that belong to the component, e.g. its chart.
	Paths []string `json:"paths"`
}

// Pins is pins.json.
type Pins struct {
	Distroless struct {
		Image  string `json:"image"`
		Tag    string `json:"tag"`
		Digest string `json:"digest"`
		Hash   string `json:"hash"`
	} `json:"distroless"`
	Gvisor struct {
		Release string            `json:"release"`
		Hash    map[string]string `json:"hash"`
	} `json:"gvisor"`
	K3s struct {
		Image   string `json:"image"`
		Channel string `json:"channel"`
		Tag     string `json:"tag"`
	} `json:"k3s"`
}

// GoModules is nix/go-modules.json: module directory to the hash of its
// dependencies, or nil for a module without external dependencies.
type GoModules map[string]*string

const goModulesFile = "nix/go-modules.json"

func LoadProject() (Project, error) { return LoadProjectFile("project.json") }

// LoadProjectFile reads a project file elsewhere, e.g. in the Nix store.
func LoadProjectFile(path string) (Project, error) {
	var p Project
	return p, readJSON(path, &p)
}

// Component returns the named component, or an error if there is none.
func (p Project) Component(name string) (Component, error) {
	c, ok := p.Components[name]
	if !ok {
		return Component{}, fmt.Errorf("unknown component %q", name)
	}
	return c, nil
}

func LoadVersions() (map[string]string, error) {
	v := map[string]string{}
	return v, readJSON("versions.json", &v)
}

func SaveVersions(v map[string]string) error { return writeJSON("versions.json", v) }

func LoadPins() (Pins, error) {
	var p Pins
	return p, readJSON("pins.json", &p)
}

func SavePins(p Pins) error { return writeJSON("pins.json", p) }

func LoadGoModules() (GoModules, error) {
	m := GoModules{}
	return m, readJSON(goModulesFile, &m)
}

func SaveGoModules(m GoModules) error { return writeJSON(goModulesFile, m) }

// Tag is the git tag of a component release, e.g. spikectl/v0.1.0.
func Tag(component, version string) string {
	return component + "/v" + version
}

// ParseTag splits a release tag into component and version.
func ParseTag(tag string) (component, version string, err error) {
	component, version, ok := strings.Cut(tag, "/v")
	if !ok || component == "" || version == "" {
		return "", "", fmt.Errorf("tag %q is not <component>/v<version>", tag)
	}
	return component, version, nil
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
