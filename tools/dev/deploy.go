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

// The services the environment runs, besides the e2e suite.
var services = []string{"spiked", "spiked-echo"}

// deploy builds and pushes every image to the cluster's registry, then
// syncs the environment.
func deploy(env string) error {
	for _, name := range append(services, "e2e") {
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

// helmfile runs helmfile with what dev/helmfile.yaml.gotmpl reads from the
// environment: per service its built chart, image tag and version
// (SPIKED_ECHO_CHART, …), the e2e image tag, and the kubeconfig.
func helmfile(env string, args ...string) error {
	versions, err := repo.LoadVersions()
	if err != nil {
		return err
	}
	var values []string
	for _, name := range services {
		img, err := buildImage(name)
		if err != nil {
			return err
		}
		chart, err := sh.Output("nix", "build", ".#"+name+"-chart", "--no-link", "--print-out-paths")
		if err != nil {
			return err
		}
		prefix := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		values = append(values,
			prefix+"_CHART="+filepath.Join(chart, name+"-"+versions[name]+".tgz"),
			prefix+"_TAG="+img.tag,
			prefix+"_VERSION="+versions[name])
	}
	e2e, err := buildImage("e2e")
	if err != nil {
		return err
	}
	// Absolute: helmfile runs helm from the directory of its file.
	config, err := filepath.Abs(kubeconfig)
	if err != nil {
		return err
	}
	values = append(values, "E2E_TAG="+e2e.tag, "KUBECONFIG="+config)
	return sh.Cmd{Env: values}.Run("helmfile",
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
