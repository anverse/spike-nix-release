package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
	"github.com/anverse/spike-nix-release/tools/internal/sh"
)

// The identity Google signs distroless images with (cosign, keyless).
const (
	distrolessSigner = "keyless@distroless.iam.gserviceaccount.com"
	distrolessIssuer = "https://accounts.google.com"
)

// updateDistroless pins the newest digest of the base image's tag:
//  1. look up the digest the tag points to now
//  2. verify Google's signature on that digest; an unsigned image is not pinned
//  3. pin the digest and the hash of the downloaded image
func updateDistroless() error {
	pins, err := repo.LoadPins()
	if err != nil {
		return err
	}
	d := &pins.Distroless
	latest, err := sh.Output("regctl", "manifest", "digest", d.Image+":"+d.Tag)
	if err != nil {
		return err
	}
	if latest == d.Digest {
		fmt.Println("distroless is current")
		return nil
	}

	fmt.Printf("distroless %s:%s -> %s\n", d.Image, d.Tag, latest)
	if _, err := sh.Output("cosign", "verify", d.Image+"@"+latest,
		"--certificate-identity", distrolessSigner, "--certificate-oidc-issuer", distrolessIssuer); err != nil {
		return fmt.Errorf("the new distroless digest is not signed by Google: %w", err)
	}
	fmt.Println("distroless: signed by", distrolessSigner)
	d.Digest, d.Hash = latest, ""
	if err := repo.SavePins(pins); err != nil {
		return err
	}
	if d.Hash, err = hashReportedByNix(".#distroless"); err != nil {
		return err
	}
	return repo.SavePins(pins)
}

// updateGvisor pins the newest gVisor release and the hashes of its
// downloads. GitHub tags look like release-20261005.0; the download path
// drops ".0".
func updateGvisor() error {
	pins, err := repo.LoadPins()
	if err != nil {
		return err
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := getJSON("https://api.github.com/repos/google/gvisor/releases/latest", &release); err != nil {
		return err
	}
	latest := strings.TrimSuffix(strings.TrimPrefix(release.TagName, "release-"), ".0")
	if latest == pins.Gvisor.Release {
		fmt.Println("gVisor is current")
		return nil
	}

	fmt.Println("gVisor ->", latest)
	for arch := range pins.Gvisor.Hash {
		url := fmt.Sprintf("https://storage.googleapis.com/gvisor/releases/release/%s/%s/gvisor.tar.bz2", latest, arch)
		if pins.Gvisor.Hash[arch], err = fileHash(url); err != nil {
			return err
		}
	}
	pins.Gvisor.Release = latest
	return repo.SavePins(pins)
}

// updateK3s pins the newest k3s of the configured release channel, by tag
// and digest. Versions look like v1.36.5+k3s1; image tags use "-" instead
// of "+".
func updateK3s() error {
	pins, err := repo.LoadPins()
	if err != nil {
		return err
	}
	var channels struct {
		Data []struct{ ID, Latest string }
	}
	if err := getJSON("https://update.k3s.io/v1-release/channels", &channels); err != nil {
		return err
	}
	for _, c := range channels.Data {
		if c.ID != pins.K3s.Channel {
			continue
		}
		latest := strings.ReplaceAll(c.Latest, "+", "-")
		digest, err := sh.Output("regctl", "manifest", "digest", pins.K3s.Image+":"+latest)
		if err != nil {
			return err
		}
		if latest == pins.K3s.Tag && digest == pins.K3s.Digest {
			fmt.Println("k3s is current")
			return nil
		}
		fmt.Println("k3s ->", latest, digest)
		pins.K3s.Tag, pins.K3s.Digest = latest, digest
		return repo.SavePins(pins)
	}
	return fmt.Errorf("no k3s channel %q", pins.K3s.Channel)
}

func getJSON(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Println("close", url+":", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
