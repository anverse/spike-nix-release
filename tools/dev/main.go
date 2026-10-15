// dev runs the development cluster: k3d with gVisor and a local registry,
// the environment from dev/helmfile.yaml.gotmpl, and the e2e suite as Helm
// tests. Run it from the repository root, through `nix run .#dev -- <command>`.
// It needs a running Docker engine.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var env string
	root := &cobra.Command{
		Use:          "dev",
		Short:        "Development cluster and e2e tests",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&env, "env", "local", "helmfile environment: local or ci")
	root.AddCommand(
		&cobra.Command{
			Use:   "up",
			Short: "Create the cluster",
			RunE:  func(*cobra.Command, []string) error { return up() },
		},
		&cobra.Command{
			Use:   "deploy",
			Short: "Build the images, push them to the cluster's registry, deploy",
			RunE:  func(*cobra.Command, []string) error { return deploy(env) },
		},
		&cobra.Command{
			Use:   "test",
			Short: "Run the Helm tests",
			RunE:  func(*cobra.Command, []string) error { return test(env) },
		},
		&cobra.Command{
			Use:   "e2e",
			Short: "Create the cluster, deploy, run the Helm tests",
			RunE:  func(*cobra.Command, []string) error { return e2e(env) },
		},
		&cobra.Command{
			Use:   "down",
			Short: "Delete the cluster",
			RunE:  func(*cobra.Command, []string) error { return down() },
		},
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func e2e(env string) error {
	if err := up(); err != nil {
		return err
	}
	if err := deploy(env); err != nil {
		return err
	}
	return test(env)
}
