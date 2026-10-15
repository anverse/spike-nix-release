// spiked stands in for agentd: a service released as a container image
// and a Helm chart.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"runtime"

	"github.com/anverse/spike-nix-release/internal/buildinfo"
)

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
		json.NewEncoder(w).Encode(map[string]string{
			"version": buildinfo.Version,
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		})
	})
	slog.Info("spiked listening", "addr", addr, "version", buildinfo.Version)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
