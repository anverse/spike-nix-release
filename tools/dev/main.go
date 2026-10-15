// dev runs the development cluster: k3d with gVisor and a local registry,
// the environment from dev/helmfile.yaml.gotmpl, and the e2e suite as Helm
// tests.
//
//	dev up                     create the cluster
//	dev deploy [-env local]    build the images, push them, helmfile sync
//	dev test [-env local]      run the Helm tests
//	dev e2e [-env local]       up, deploy, test
//	dev down                   delete the cluster
//
// Run from the repository root, through `nix run .#dev -- …`. Needs a
// running Docker engine.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

const (
	cluster    = "spike-0011"
	kubeconfig = ".cache/kubeconfig"
	// The registry as the host sees it; pods pull from spike-registry:5000.
	registry = "127.0.0.1:5111"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: dev up|deploy|test|e2e|down [-env local|ci]"))
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	env := fs.String("env", "local", "helmfile environment: local or ci")
	fs.Parse(os.Args[2:])

	var err error
	switch os.Args[1] {
	case "up":
		err = up()
	case "deploy":
		err = deploy(*env)
	case "test":
		err = test(*env)
	case "e2e":
		if err = up(); err == nil {
			if err = deploy(*env); err == nil {
				err = test(*env)
			}
		}
	case "down":
		err = down()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func up() error {
	if project.Run("k3d", "cluster", "list", cluster) == nil {
		fmt.Println("cluster", cluster, "exists")
		return nil
	}

	// gVisor for the architecture of the Docker host, copied out of the Nix
	// store: Docker on macOS cannot mount /nix/store.
	arch, err := project.Output("docker", "info", "--format", "{{.Architecture}}")
	if err != nil {
		return err
	}
	switch arch {
	case "aarch64", "arm64":
		arch = "aarch64"
	case "x86_64", "amd64":
		arch = "x86_64"
	default:
		return fmt.Errorf("no gVisor for Docker architecture %q", arch)
	}
	gvisor, err := project.Output("nix", "build", ".#gvisor-"+arch, "--no-link", "--print-out-paths")
	if err != nil {
		return err
	}
	os.RemoveAll(".cache/gvisor")
	os.MkdirAll(".cache", 0o755)
	if err := project.Run("cp", "-RL", gvisor+"/bin", ".cache/gvisor"); err != nil {
		return err
	}
	if err := project.Run("chmod", "-R", "u+w,a+rX", ".cache/gvisor"); err != nil {
		return err
	}

	k3s, err := k3sImage()
	if err != nil {
		return err
	}
	root, _ := os.Getwd()
	clusterEnv := []string{
		"K3S_IMAGE=" + k3s,
		"GVISOR_DIR=" + filepath.Join(root, ".cache/gvisor"),
		"DEV_DIR=" + filepath.Join(root, "dev"),
	}
	if err := project.RunWith(clusterEnv, "", "k3d", "cluster", "create", "--config", "dev/k3d.yaml"); err != nil {
		return err
	}
	config, err := project.Output("k3d", "kubeconfig", "get", cluster)
	if err != nil {
		return err
	}
	if err := os.WriteFile(kubeconfig, []byte(config+"\n"), 0o600); err != nil {
		return err
	}
	if err := project.Run("kubectl", "--kubeconfig", kubeconfig, "wait", "--for=condition=Ready", "node", "--all", "--timeout=120s"); err != nil {
		return err
	}
	// Pods resolve services through the cluster DNS. k3s creates CoreDNS a
	// few seconds after the node is ready, so wait until it exists and runs.
	deadline := time.Now().Add(3 * time.Minute)
	for {
		err := project.Run("kubectl", "--kubeconfig", kubeconfig, "-n", "kube-system",
			"rollout", "status", "deployment/coredns", "--timeout=60s")
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Second)
	}
}

func down() error {
	os.Remove(kubeconfig)
	return project.Run("k3d", "cluster", "delete", cluster)
}

// deploy pushes the images to the local registry and syncs the environment.
func deploy(env string) error {
	for _, name := range []string{"spiked", "e2e"} {
		layout, ref, tag, err := image(name)
		if err != nil {
			return err
		}
		if err := project.Run("regctl", "image", "copy", "--host", "reg="+registry+",tls=disabled",
			"ocidir://"+layout+":"+ref, registry+"/"+name+":"+tag); err != nil {
			return err
		}
	}
	return helmfile(env, "sync")
}

func test(env string) error {
	return helmfile(env, "test", "--logs")
}

// helmfile runs helmfile with the values dev/helmfile.yaml.gotmpl reads from
// the environment: image tags, the expected version, the cluster's
// kubeconfig.
func helmfile(env string, args ...string) error {
	_, _, spikedTag, err := image("spiked")
	if err != nil {
		return err
	}
	_, _, e2eTag, err := image("e2e")
	if err != nil {
		return err
	}
	versions, err := project.LoadVersions(".")
	if err != nil {
		return err
	}
	// Absolute: helmfile runs helm from the directory of its file.
	config, err := filepath.Abs(kubeconfig)
	if err != nil {
		return err
	}
	values := []string{
		"SPIKED_TAG=" + spikedTag,
		"E2E_TAG=" + e2eTag,
		"SPIKED_VERSION=" + versions["spiked"],
		"KUBECONFIG=" + config,
	}
	return project.RunWith(values, "", "helmfile",
		append([]string{"--file", "dev/helmfile.yaml.gotmpl", "--environment", env}, args...)...)
}

// image builds an image with Nix and returns its layout, its tag in the
// layout, and a tag for the registry derived from its digest, so every new
// build gets a new tag.
func image(name string) (layout, ref, tag string, err error) {
	layout, err = project.Output("nix", "build", ".#"+name+"-image", "--no-link", "--print-out-paths")
	if err != nil {
		return "", "", "", err
	}
	var index struct {
		Manifests []struct {
			Digest      string            `json:"digest"`
			Annotations map[string]string `json:"annotations"`
		} `json:"manifests"`
	}
	b, err := os.ReadFile(filepath.Join(layout, "index.json"))
	if err != nil {
		return "", "", "", err
	}
	if err := json.Unmarshal(b, &index); err != nil || len(index.Manifests) != 1 {
		return "", "", "", fmt.Errorf("%s: want one image", layout)
	}
	m := index.Manifests[0]
	return layout, m.Annotations["org.opencontainers.image.ref.name"], strings.TrimPrefix(m.Digest, "sha256:")[:12], nil
}

func k3sImage() (string, error) {
	var pins struct {
		K3s struct{ Image, Tag string } `json:"k3s"`
	}
	b, err := os.ReadFile("pins.json")
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(b, &pins); err != nil {
		return "", err
	}
	return pins.K3s.Image + ":" + pins.K3s.Tag, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "dev:", err)
	os.Exit(1)
}
