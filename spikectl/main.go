// spikectl stands in for agentctl: a command-line tool released for
// Linux, macOS and Windows.
package main

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/mod/semver"
)

// version is set at link time by the Nix build.
var buildVersion = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		valid := semver.IsValid("v" + buildVersion)
		fmt.Printf("spikectl %s %s/%s semver=%t\n", buildVersion, runtime.GOOS, runtime.GOARCH, valid)
		return
	}
	fmt.Fprintln(os.Stderr, "usage: spikectl version (or --version)")
	os.Exit(2)
}
