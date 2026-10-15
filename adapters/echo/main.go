// spiked-echo stands in for an agentd adapter: a separate service with
// dependencies of its own (here github.com/google/uuid), which no other
// component carries.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"runtime"

	"github.com/google/uuid"

	"github.com/anverse/spike-nix-release/api/version"
)

// buildVersion is set at link time by the Nix build.
var buildVersion = "dev"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(version.Info{
			Component: "spiked-echo",
			Version:   buildVersion,
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		})
	})
	mux.HandleFunc("GET /echo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"id":      uuid.NewString(),
			"message": r.URL.Query().Get("message"),
		})
	})
	slog.Info("spiked-echo listening", "addr", ":8080", "version", buildVersion)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
