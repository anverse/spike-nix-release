package main

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

var reportedHash = regexp.MustCompile(`got:\s+(sha256-\S+)`)

// hashReportedByNix builds a download whose pinned hash is empty. Nix
// refuses it and reports the hash it got, which becomes the new pin.
func hashReportedByNix(installable string) (string, error) {
	out, _ := sh.Output("sh", "-c", "nix build "+installable+" --no-link 2>&1 || true")
	match := reportedHash.FindStringSubmatch(out)
	if match == nil {
		return "", fmt.Errorf("nix reported no hash for %s:\n%s", installable, out)
	}
	return match[1], nil
}

// fileHash downloads a single file into the Nix store and returns its hash.
func fileHash(url string) (string, error) {
	out, err := sh.Output("nix", "store", "prefetch-file", "--json", "--hash-type", "sha512", url)
	if err != nil {
		return "", err
	}
	var prefetched struct{ Hash string }
	if err := json.Unmarshal([]byte(out), &prefetched); err != nil {
		return "", err
	}
	return prefetched.Hash, nil
}
