// release prepares, tags and publishes component releases (ADR-0024). Run
// it from the repository root, through `nix run .#release -- <command>`.
//
// A release goes:
//
//	release prepare <component> <version>   pull request with the new version
//	(review, CI, merge)
//	release tag <component> <summary>       tag the merged commit; starts the workflow
//	release verify <tag>                    (workflow) the tag is on main and matches
//	release dist <component> <tag>          (workflow) build into dist/, write the notes
//	release publish <tag>                   (workflow) publish dist/<component>
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "release",
		Short:        "Prepare, tag and publish component releases",
		SilenceUsage: true,
	}
	root.AddCommand(
		&cobra.Command{
			Use:   "prepare <component> <version>",
			Short: "Open a pull request that sets the component's new version",
			Args:  cobra.ExactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return prepare(args[0], args[1]) },
		},
		&cobra.Command{
			Use:   "tag <component> <summary>",
			Short: "Tag the merged release on main and push the tag, which starts the release workflow",
			Long:  "The summary is the tag's message and starts the release notes.",
			Args:  cobra.ExactArgs(2),
			RunE:  func(_ *cobra.Command, args []string) error { return tagRelease(args[0], args[1]) },
		},
		&cobra.Command{
			Use:   "notes <component> [<tag>]",
			Short: "Print the release notes; without a tag, of the unreleased changes",
			Args:  cobra.RangeArgs(1, 2),
			RunE: func(cmd *cobra.Command, args []string) error {
				text, err := notes(args[0], optionalArg(args, 1))
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(cmd.OutOrStdout(), text)
				return err
			},
		},
		&cobra.Command{
			Use:   "dist <component> [<tag>]",
			Short: "Build a release into dist/<component>, with its notes, without publishing",
			Args:  cobra.RangeArgs(1, 2),
			RunE:  func(_ *cobra.Command, args []string) error { return dist(args[0], optionalArg(args, 1)) },
		},
		&cobra.Command{
			Use:   "verify <tag>",
			Short: "Check that a release tag is on main and matches versions.json (CI)",
			Args:  cobra.ExactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return verify(args[0]) },
		},
		&cobra.Command{
			Use:   "login",
			Short: "Log in to ghcr.io with GITHUB_ACTOR and GITHUB_TOKEN (CI)",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return login() },
		},
		&cobra.Command{
			Use:   "publish <tag>",
			Short: "Publish the release in dist/<component>; safe to repeat (CI)",
			Args:  cobra.ExactArgs(1),
			RunE:  func(_ *cobra.Command, args []string) error { return publish(args[0]) },
		},
		&cobra.Command{
			Use:   "check",
			Short: "Check that go.work, nix/go-modules.json, project.json and versions.json agree",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return checkConfig() },
		},
		manifestsCommand(),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
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
