package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// A repository with two components, each its own Go module using a shared
// api module: each one's notes list only its own commits; api changes
// appear in both.
func TestNotes(t *testing.T) {
	t.Chdir(t.TempDir())
	write := func(path, text string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.org",
			"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit := func(path, message string) {
		write(path, message)
		git("add", "-A")
		git("commit", "-q", "-m", message)
	}

	write("project.json", `{"components": {
		"cli": {"kind": "cli", "module": "cli"},
		"svc": {"kind": "service", "module": "svc", "paths": ["charts/svc"]},
		"idle": {"kind": "service", "module": "idle"}}}`)
	write("nix/go-modules.json", `{
		"api": {"hash": null, "local": []},
		"cli": {"hash": null, "local": []},
		"svc": {"hash": null, "local": ["api"]},
		"idle": {"hash": null, "local": []}}`)
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "set up the modules")
	commit("cli/main.go", "start the cli")
	commit("svc/main.go", "start the service")
	commit("api/api.go", "define the api")
	commit("versions.json", "release cli v0.1.0")
	git("tag", "-a", "cli/v0.1.0", "-m", "First cli release.")
	commit("cli/notes.txt", "release notes: fix a typo")
	commit("cli/main.go", "improve the cli")
	commit("charts/svc/values.yaml", "document a chart value")
	commit("versions.json", "release cli v0.2.0")
	git("tag", "-a", "cli/v0.2.0", "-m", "Second cli release.")

	tests := []struct{ component, tag, want string }{
		{"cli", "cli/v0.1.0", "First cli release.\n\n## Changes\n\n- start the cli (H)\n"},
		{"cli", "cli/v0.2.0", "Second cli release.\n\n## Changes\n\n- release notes: fix a typo (H)\n- improve the cli (H)\n"},
		{"idle", "", "Unreleased changes.\n\n## Changes\n\nNo changes to this component since its previous release.\n"},
		{"svc", "", "Unreleased changes.\n\n## Changes\n\n- start the service (H)\n- define the api (H)\n- document a chart value (H)\n"},
	}
	for _, tt := range tests {
		text, err := notes(tt.component, tt.tag)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := hideHashes(text), tt.want; got != want {
			t.Errorf("notes(%s, %s)=\n%s\nwant=\n%s", tt.component, tt.tag, got, want)
		}
	}
}

var commitHash = regexp.MustCompile(`\([0-9a-f]{7,}\)`)

func hideHashes(s string) string { return commitHash.ReplaceAllString(s, "(H)") }

// A signed tag's message ends with its signature; the notes must not show it.
func TestTagMessage(t *testing.T) {
	raw := "object 0123\ntype commit\ntag cli/v0.1.0\ntagger t <t@example.org> 0 +0000\n\n" +
		"First release.\n\nWith a body.\n-----BEGIN SSH SIGNATURE-----\nU1NIU0lH\n-----END SSH SIGNATURE-----\n"
	if got, want := tagMessage(raw), "First release.\n\nWith a body."; got != want {
		t.Errorf("tagMessage=%q, want=%q", got, want)
	}
}
