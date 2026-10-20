package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

const (
	cluster    = "spike-0011"
	kubeconfig = ".cache/kubeconfig"
)

// up creates the cluster, unless it exists:
//  1. copy gVisor for the Docker host's architecture out of the Nix store
//     (Docker on macOS cannot mount /nix/store)
//  2. create the k3d cluster from dev/k3d.yaml, with k3s from pins.json
//     (tag and digest)
//  3. wait until the node and the cluster DNS are ready
func up() error {
	if sh.Run("k3d", "cluster", "list", cluster) == nil {
		fmt.Println("cluster", cluster, "exists")
		return nil
	}
	if err := copyGvisor(".cache/gvisor"); err != nil {
		return err
	}
	if err := createCluster(".cache/gvisor"); err != nil {
		return err
	}
	return waitUntilReady()
}

func down() error {
	if err := os.Remove(kubeconfig); err != nil && !os.IsNotExist(err) {
		return err
	}
	return sh.Run("k3d", "cluster", "delete", cluster)
}

func copyGvisor(to string) error {
	arch, err := dockerArchitecture()
	if err != nil {
		return err
	}
	gvisor, err := sh.Output("nix", "build", ".#gvisor-"+arch, "--no-link", "--print-out-paths")
	if err != nil {
		return err
	}
	if err := os.RemoveAll(to); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if err := sh.Run("cp", "-RL", gvisor+"/bin", to); err != nil {
		return err
	}
	return sh.Run("chmod", "-R", "u+w,a+rX", to)
}

// dockerArchitecture is the Docker host's architecture as gVisor names it.
func dockerArchitecture() (string, error) {
	arch, err := sh.Output("docker", "info", "--format", "{{.Architecture}}")
	if err != nil {
		return "", err
	}
	switch arch {
	case "aarch64", "arm64":
		return "aarch64", nil
	case "x86_64", "amd64":
		return "x86_64", nil
	}
	return "", fmt.Errorf("no gVisor for Docker architecture %q", arch)
}

// createCluster creates the cluster; dev/k3d.yaml reads K3S_IMAGE,
// GVISOR_DIR and DEV_DIR from the environment.
func createCluster(gvisor string) error {
	pins, err := repo.LoadPins()
	if err != nil {
		return err
	}
	gvisorDir, _ := filepath.Abs(gvisor)
	devDir, _ := filepath.Abs("dev")
	withSettings := sh.Cmd{Env: []string{
		"K3S_IMAGE=" + pins.K3s.Image + ":" + pins.K3s.Tag + "@" + pins.K3s.Digest,
		"GVISOR_DIR=" + gvisorDir,
		"DEV_DIR=" + devDir,
	}}
	if err := withSettings.Run("k3d", "cluster", "create", "--config", "dev/k3d.yaml"); err != nil {
		return err
	}
	config, err := sh.Output("k3d", "kubeconfig", "get", cluster)
	if err != nil {
		return err
	}
	return os.WriteFile(kubeconfig, []byte(config+"\n"), 0o600)
}

// waitUntilReady waits for the node, then for CoreDNS: pods resolve
// services through it, and k3s creates it a few seconds after the node is
// ready.
func waitUntilReady() error {
	if err := kubectl("wait", "--for=condition=Ready", "node", "--all", "--timeout=120s"); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Minute)
	for {
		err := kubectl("-n", "kube-system", "rollout", "status", "deployment/coredns", "--timeout=60s")
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Second)
	}
}

func kubectl(args ...string) error {
	return sh.Run("kubectl", append([]string{"--kubeconfig", kubeconfig}, args...)...)
}
