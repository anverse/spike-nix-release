package main

import "github.com/anverse/spike-nix-release/tools/internal/sh"

// The command-line tools the release steps call, so the steps read like
// the commands a person would type.

func git(args ...string) error                 { return sh.Run("git", args...) }
func gitOutput(args ...string) (string, error) { return sh.Output("git", args...) }
func nix(args ...string) error                 { return sh.Run("nix", args...) }
func nixOutput(args ...string) (string, error) { return sh.Output("nix", args...) }
func gh(args ...string) error                  { return sh.Run("gh", args...) }
func regctl(args ...string) error              { return sh.Run("regctl", args...) }
func helm(args ...string) error                { return sh.Run("helm", args...) }
