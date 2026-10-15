package main

import "github.com/anverse/spike-nix-release/tools/internal/sh"

// updateFlake moves nixpkgs to the newest commit of its branch.
func updateFlake() error {
	return sh.Run("nix", "flake", "update")
}
