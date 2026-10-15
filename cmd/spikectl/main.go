// spikectl stands in for agentctl: a command-line tool released for
// Linux, macOS and Windows.
package main

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/mod/semver"

	"github.com/anverse/spike-nix-release/internal/buildinfo"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		valid := semver.IsValid("v" + buildinfo.Version)
		fmt.Printf("spikectl %s %s/%s semver=%t\n", buildinfo.Version, runtime.GOOS, runtime.GOARCH, valid)
		return
	}
	fmt.Fprintln(os.Stderr, "usage: spikectl version")
	os.Exit(2)
}
