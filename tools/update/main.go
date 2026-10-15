// update moves every pinned dependency to its newest version and fixes the
// hashes that pin its content:
//
//	flake       nixpkgs in flake.lock, and with it every tool
//	go          Go modules in go.mod and go.sum, and the vendorHash
//	distroless  the base image digest in pins.json
//	gvisor      the gVisor release in pins.json
//	k3s         the k3s version of the configured channel in pins.json
//
//	update [name ...]    all of them without a name
//
// Run from the repository root, through `nix run .#update`.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

// Pins is pins.json: external artifacts the build downloads.
type Pins struct {
	Distroless struct {
		Image  string `json:"image"`
		Tag    string `json:"tag"`
		Digest string `json:"digest"`
		Hash   string `json:"hash"`
	} `json:"distroless"`
	Gvisor struct {
		Release string            `json:"release"`
		Hash    map[string]string `json:"hash"`
	} `json:"gvisor"`
	K3s struct {
		Image   string `json:"image"`
		Channel string `json:"channel"`
		Tag     string `json:"tag"`
	} `json:"k3s"`
}

var updates = map[string]func(*Pins) error{
	"flake":      updateFlake,
	"go":         updateGo,
	"distroless": updateDistroless,
	"gvisor":     updateGvisor,
	"k3s":        updateK3s,
}

var order = []string{"flake", "go", "distroless", "gvisor", "k3s"}

func main() {
	names := os.Args[1:]
	if len(names) == 0 {
		names = order
	}
	var pins Pins
	if err := readJSON("pins.json", &pins); err != nil {
		fail(err)
	}
	for _, name := range names {
		update, ok := updates[name]
		if !ok {
			fail(fmt.Errorf("unknown update %q; known: %s", name, strings.Join(order, ", ")))
		}
		if err := update(&pins); err != nil {
			fail(fmt.Errorf("%s: %w", name, err))
		}
	}
}

func updateFlake(*Pins) error {
	return project.Run("nix", "flake", "update")
}

// updateGo updates the Go modules, then the vendorHash all Go builds share.
func updateGo(*Pins) error {
	os.Setenv("CGO_ENABLED", "0")
	for _, s := range [][]string{
		{"go", "get", "-u", "./..."},
		{"go", "mod", "tidy"},
		{"nix-update", "--flake", "--version=skip", "spikectl"},
	} {
		if err := project.Run(s[0], s[1:]...); err != nil {
			return err
		}
	}
	return nil
}

// updateDistroless pins the newest digest of the base image tag. Its hash
// covers the downloaded image layout: Nix reports it when it builds the
// download with an empty hash.
func updateDistroless(p *Pins) error {
	d := &p.Distroless
	latest, err := project.Output("regctl", "manifest", "digest", d.Image+":"+d.Tag)
	if err != nil {
		return err
	}
	if latest == d.Digest {
		fmt.Println("distroless is current")
		return nil
	}
	fmt.Printf("distroless %s:%s -> %s\n", d.Image, d.Tag, latest)
	d.Digest, d.Hash = latest, ""
	if err := writeJSON("pins.json", p); err != nil {
		return err
	}
	d.Hash, err = hashReportedByNix(".#distroless")
	if err != nil {
		return err
	}
	if err := writeJSON("pins.json", p); err != nil {
		return err
	}
	return project.Run("nix", "build", ".#distroless", "--no-link")
}

// updateGvisor pins the newest gVisor release and the hashes of its
// downloads. GitHub tags look like release-20261005.0; the download path
// drops ".0".
func updateGvisor(p *Pins) error {
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := getJSON("https://api.github.com/repos/google/gvisor/releases/latest", &release); err != nil {
		return err
	}
	latest := strings.TrimSuffix(strings.TrimPrefix(release.TagName, "release-"), ".0")
	if latest == p.Gvisor.Release {
		fmt.Println("gVisor is current")
		return nil
	}
	fmt.Println("gVisor ->", latest)
	for arch := range p.Gvisor.Hash {
		url := fmt.Sprintf("https://storage.googleapis.com/gvisor/releases/release/%s/%s/gvisor.tar.bz2", latest, arch)
		out, err := project.Output("nix", "store", "prefetch-file", "--json", "--hash-type", "sha512", url)
		if err != nil {
			return err
		}
		var prefetched struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal([]byte(out), &prefetched); err != nil {
			return err
		}
		p.Gvisor.Hash[arch] = prefetched.Hash
	}
	p.Gvisor.Release = latest
	return writeJSON("pins.json", p)
}

// updateK3s pins the newest k3s of the configured release channel. Versions
// look like v1.36.5+k3s1; image tags use "-" instead of "+".
func updateK3s(p *Pins) error {
	var channels struct {
		Data []struct {
			ID     string `json:"id"`
			Latest string `json:"latest"`
		} `json:"data"`
	}
	if err := getJSON("https://update.k3s.io/v1-release/channels", &channels); err != nil {
		return err
	}
	for _, c := range channels.Data {
		if c.ID != p.K3s.Channel {
			continue
		}
		latest := strings.ReplaceAll(c.Latest, "+", "-")
		if latest == p.K3s.Tag {
			fmt.Println("k3s is current")
			return nil
		}
		fmt.Println("k3s ->", latest)
		p.K3s.Tag = latest
		return writeJSON("pins.json", p)
	}
	return fmt.Errorf("no k3s channel %q", p.K3s.Channel)
}

var gotHash = regexp.MustCompile(`got:\s+(sha256-\S+)`)

// hashReportedByNix builds a fixed-output download whose hash is empty; Nix
// refuses it and reports the hash it got.
func hashReportedByNix(installable string) (string, error) {
	out, _ := project.Output("sh", "-c", "nix build "+installable+" --no-link 2>&1 || true")
	m := gotHash.FindStringSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("nix reported no hash:\n%s", out)
	}
	return m[1], nil
}

func getJSON(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "update:", err)
	os.Exit(1)
}
