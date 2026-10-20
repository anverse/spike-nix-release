// spikectl stands in for agentctl: a command-line tool released for
// Linux, macOS and Windows.
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"golang.org/x/mod/semver"
)

// buildVersion is set at link time by the Nix build.
var buildVersion = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		v := currentVersion()
		fmt.Printf("spikectl %s %s/%s semver=%t\n", v, runtime.GOOS, runtime.GOARCH, semver.IsValid("v"+v))
		return
	}
	fmt.Fprintln(os.Stderr, "usage: spikectl version (or --version)")
	os.Exit(2)
}

// currentVersion is the version set by the release build. A build with
// `go install …@v0.3.0` has none and reports the module version instead.
func currentVersion() string {
	if buildVersion != "dev" {
		return buildVersion
	}
	if info, ok := debug.ReadBuildInfo(); ok && semver.IsValid(info.Main.Version) {
		return info.Main.Version[1:]
	}
	return buildVersion
}
