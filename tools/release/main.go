// release cuts and publishes component releases (ADR-0024). Run it from the
// repository root, through `nix run .#release -- <command>`.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "release",
		Short:        "Cut and publish component releases",
		SilenceUsage: true,
	}
	root.AddCommand(
		cutCommand(),
		notesCommand(),
		distCommand(),
		loginCommand(),
		publishCommand(),
		manifestsCommand(),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func cutCommand() *cobra.Command {
	var noPush bool
	cmd := &cobra.Command{
		Use:   "cut <component> <version> <summary>",
		Short: "Write the version, commit, tag and push a release",
		Long:  "The pushed tag starts the release workflow. The summary starts the release notes.",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			return cut(args[0], args[1], args[2], !noPush)
		},
	}
	cmd.Flags().BoolVar(&noPush, "no-push", false, "commit and tag, but do not push")
	return cmd
}

func notesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "notes <component> [<tag>]",
		Short: "Print the release notes; without a tag, of the unreleased changes",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := notes(args[0], optionalArg(args, 1))
			fmt.Fprint(cmd.OutOrStdout(), text)
			return err
		},
	}
}

func distCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "dist <component> [<tag>]",
		Short: "Build a release into dist/<component>, with its notes, without publishing",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			return dist(args[0], optionalArg(args, 1))
		},
	}
}

func loginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to ghcr.io with GITHUB_ACTOR and GITHUB_TOKEN (CI)",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return login() },
	}
}

func publishCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "publish <tag>",
		Short: "Publish the release of a pushed tag (CI)",
		Args:  cobra.ExactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return publish(args[0]) },
	}
}

func manifestsCommand() *cobra.Command {
	var m manifestsInput
	cmd := &cobra.Command{
		Use:   "manifests",
		Short: "Render the Homebrew formula and the Scoop manifest (used by the Nix build)",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return renderManifests(m) },
	}
	cmd.Flags().StringVar(&m.Component, "component", "", "component name")
	cmd.Flags().StringVar(&m.Version, "version", "", "version")
	cmd.Flags().StringVar(&m.Project, "project", "project.json", "the project file")
	cmd.Flags().StringVar(&m.Archives, "archives", "", "directory with the archives and checksums.txt")
	cmd.Flags().StringVar(&m.Templates, "templates", "", "directory with <tool>/<component>.*.tmpl")
	cmd.Flags().StringVar(&m.Out, "out", "", "output directory")
	return cmd
}

func optionalArg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}
