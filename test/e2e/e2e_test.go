// Package e2e runs inside the cluster as a Helm test against the deployed
// services. SPIKED_URL and ECHO_URL name them, SPIKED_VERSION and
// ECHO_VERSION the versions they must report.
package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/anverse/spike-nix-release/api/version"
)

func TestSpiked(t *testing.T) {
	url := serviceURL(t, "SPIKED_URL")
	get(t, url+"/healthz")
	checkVersion(t, url, "spiked", os.Getenv("SPIKED_VERSION"))
}

func TestEcho(t *testing.T) {
	url := serviceURL(t, "ECHO_URL")
	checkVersion(t, url, "spiked-echo", os.Getenv("ECHO_VERSION"))

	var echoed map[string]string
	decode(t, get(t, url+"/echo?message=hello"), &echoed)
	if got, want := echoed["message"], "hello"; got != want {
		t.Errorf("message=%s, want=%s", got, want)
	}
	if echoed["id"] == "" {
		t.Error("the echo has no id")
	}
}

func checkVersion(t *testing.T, url, component, wantVersion string) {
	t.Helper()
	var info version.Info
	decode(t, get(t, url+"/version"), &info)
	t.Logf("%s reports %+v", component, info)
	if got, want := info.Component, component; got != want {
		t.Errorf("component=%s, want=%s", got, want)
	}
	if wantVersion != "" {
		if got, want := info.Version, wantVersion; got != want {
			t.Errorf("version=%s, want=%s", got, want)
		}
	}
}

func serviceURL(t *testing.T, variable string) string {
	t.Helper()
	url := os.Getenv(variable)
	if url == "" {
		t.Skip(variable + " not set; runs as a Helm test in the cluster")
	}
	return url
}

// get requests a URL. A pod that is ready is not reachable through its
// Service at once; the first requests may be refused until the cluster has
// programmed the route, so get retries for up to a minute.
func get(t *testing.T, url string) *http.Response {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		resp, err := http.Get(url)
		if err == nil {
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Error(err)
				}
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: status %d", url, resp.StatusCode)
			}
			return resp
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
	}
}

func decode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}
