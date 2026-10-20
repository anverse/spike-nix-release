// spiked stands in for agentd: a service released as a container image
// and a Helm chart.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/anverse/spike-nix-release/api/version"
)

// buildVersion is set at link time by the Nix build.
var buildVersion = "dev"

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, version.Info{
			Component: "spiked",
			Version:   buildVersion,
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		})
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("spiked listening", "addr", addr, "version", buildVersion)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}

func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err)
	}
}
