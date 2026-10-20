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
	"time"

	"github.com/google/uuid"

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
			Component: "spiked-echo",
			Version:   buildVersion,
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		})
	})
	mux.HandleFunc("GET /echo", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]string{
			"id":      uuid.NewString(),
			"message": r.URL.Query().Get("message"),
		})
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("echo adapter listening", "addr", addr, "version", buildVersion)
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
