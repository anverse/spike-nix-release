// update moves pinned dependencies to their newest versions and fixes the
// hashes that pin their content. Run it from the repository root, through
// `nix run .#update [-- <what>]`; without an argument it updates everything.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "update",
		Short:        "Update every pinned dependency",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			for _, update := range []func() error{updateFlake, updateGo, updateDistroless, updateGvisor, updateK3s} {
				if err := update(); err != nil {
					return err
				}
			}
			return nil
		},
	}
	root.AddCommand(
		command("flake", "nixpkgs in flake.lock, and with it every tool", updateFlake),
		command("go", "the Go modules of every module in go.work, and their hashes", updateGo),
		command("distroless", "the base image digest in pins.json", updateDistroless),
		command("gvisor", "the gVisor release in pins.json", updateGvisor),
		command("k3s", "the k3s version of the configured channel in pins.json", updateK3s),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func command(name, what string, update func() error) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: "Update " + what,
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return update() },
	}
}
