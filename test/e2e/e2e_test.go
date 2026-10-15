// Package e2e runs inside the cluster as a Helm test against the deployed
// release. SPIKED_URL names the service, SPIKED_VERSION the expected version.
package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

// get requests path from the service. A pod that is ready is not reachable
// through its Service at once; the first requests may be refused until the
// cluster has programmed the route, so get retries for up to a minute.
func get(t *testing.T, path string) *http.Response {
	t.Helper()
	if os.Getenv("SPIKED_URL") == "" {
		t.Skip("SPIKED_URL not set; runs as a Helm test in the cluster")
	}
	deadline := time.Now().Add(time.Minute)
	for {
		resp, err := http.Get(os.Getenv("SPIKED_URL") + path)
		if err == nil {
			t.Cleanup(func() { resp.Body.Close() })
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: status %d", path, resp.StatusCode)
			}
			return resp
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
	}
}

func TestHealth(t *testing.T) {
	get(t, "/healthz")
}

func TestVersion(t *testing.T) {
	var v map[string]string
	if err := json.NewDecoder(get(t, "/version").Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	t.Logf("spiked reports %v", v)
	if want := os.Getenv("SPIKED_VERSION"); want != "" && v["version"] != want {
		t.Fatalf("version %q, want %q", v["version"], want)
	}
}
