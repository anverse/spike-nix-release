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

// The cluster's registry as the host sees it; pods pull from
// spike-registry:5000.
const registry = "127.0.0.1:5111"

// The images the environment runs.
var images = []string{"spiked", "e2e"}

// deploy pushes the images to the cluster's registry, then syncs the
// environment.
func deploy(env string) error {
	for _, name := range images {
		img, err := buildImage(name)
		if err != nil {
			return err
		}
		if err := sh.Run("regctl", "image", "copy", "--host", "reg="+registry+",tls=disabled",
			"ocidir://"+img.layout+":"+img.ref, registry+"/"+name+":"+img.tag); err != nil {
			return err
		}
	}
	return helmfile(env, "sync")
}

func test(env string) error {
	return helmfile(env, "test", "--logs")
}

// helmfile runs helmfile with the values dev/helmfile.yaml.gotmpl reads
// from the environment: image tags, the expected version, the kubeconfig.
func helmfile(env string, args ...string) error {
	spiked, err := buildImage("spiked")
	if err != nil {
		return err
	}
	e2e, err := buildImage("e2e")
	if err != nil {
		return err
	}
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	// Absolute: helmfile runs helm from the directory of its file.
	config, _ := filepath.Abs(kubeconfig)
	withValues := sh.Cmd{Env: []string{
		"SPIKED_TAG=" + spiked.tag,
		"E2E_TAG=" + e2e.tag,
		"SPIKED_VERSION=" + versions["spiked"],
		"KUBECONFIG=" + config,
	}}
	return withValues.Run("helmfile",
		append([]string{"--file", "dev/helmfile.yaml.gotmpl", "--environment", env}, args...)...)
}

// image is a built image: its layout in the Nix store, its tag in the
// layout, and the tag to push it under, taken from its digest so every new
// build gets a new tag.
type image struct{ layout, ref, tag string }

func buildImage(name string) (image, error) {
	layout, err := sh.Output("nix", "build", ".#"+name+"-image", "--no-link", "--print-out-paths")
	if err != nil {
		return image{}, err
	}
	var index struct {
		Manifests []struct {
			Digest      string
			Annotations map[string]string
		}
	}
	b, err := os.ReadFile(filepath.Join(layout, "index.json"))
	if err != nil {
		return image{}, err
	}
	if err := json.Unmarshal(b, &index); err != nil || len(index.Manifests) != 1 {
		return image{}, fmt.Errorf("%s: want one image", layout)
	}
	m := index.Manifests[0]
	return image{
		layout: layout,
		ref:    m.Annotations["org.opencontainers.image.ref.name"],
		tag:    strings.TrimPrefix(m.Digest, "sha256:")[:12],
	}, nil
}
