// spiked stands in for agentd: a service released as a container image
// and a Helm chart.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"runtime"

	"github.com/anverse/spike-nix-release/api/version"
)

// buildVersion is set at link time by the Nix build.
var buildVersion = "dev"

func main() {
	addr := os.Getenv("SPIKED_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(version.Info{
			Component: "spiked",
			Version:   buildVersion,
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		})
	})
	slog.Info("spiked listening", "addr", addr, "version", buildVersion)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
