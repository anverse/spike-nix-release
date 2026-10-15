// release cuts and publishes component releases (ADR-0024).
//
//	release cut <component> <version> <message> [-no-push]
//	release notes <component> [<tag>]
//	release dist <component> [<tag>]
//	release login
//	release publish <tag>
//	release manifests -component C -version V -archives DIR -templates DIR -out DIR
//
// Run from the repository root, through `nix run .#release -- …`.
package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `usage:
  release cut <component> <version> <message> [-no-push]   bump, commit, tag, push
  release notes <component> [<tag>]                         release notes to stdout
  release dist <component> [<tag>]                          build into dist/<component>, with notes
  release login                                             log in to ghcr.io (CI: GITHUB_ACTOR, GITHUB_TOKEN)
  release publish <tag>                                     publish a pushed tag (CI)
  release manifests ...                                     render Homebrew and Scoop manifests (Nix build)`

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("%s", usage))
	}
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "cut":
		fs := flag.NewFlagSet("cut", flag.ExitOnError)
		noPush := fs.Bool("no-push", false, "commit and tag, but do not push")
		fs.Parse(reorder(args))
		if fs.NArg() != 3 {
			fail(fmt.Errorf("%s", usage))
		}
		err = cut(fs.Arg(0), fs.Arg(1), fs.Arg(2), !*noPush)
	case "notes":
		var text string
		text, err = notes(arg(args, 0), optional(args, 1))
		fmt.Print(text)
	case "dist":
		err = dist(arg(args, 0), optional(args, 1))
	case "login":
		err = login()
	case "publish":
		err = publish(arg(args, 0))
	case "manifests":
		err = manifestsCommand(args)
	default:
		err = fmt.Errorf("%s", usage)
	}
	if err != nil {
		fail(err)
	}
}

func arg(args []string, i int) string {
	if i >= len(args) {
		fail(fmt.Errorf("%s", usage))
	}
	return args[i]
}

func optional(args []string, i int) string {
	if i >= len(args) {
		return ""
	}
	return args[i]
}

// reorder moves flags in front of positional arguments, so -no-push may
// come last.
func reorder(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "release:", err)
	os.Exit(1)
}
