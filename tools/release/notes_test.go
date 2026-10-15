package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A repository with two components and releases of both: each one's notes
// list only its own commits, shared changes appear in both.
func TestNotes(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	write := func(path, text string) {
		os.MkdirAll(filepath.Dir(path), 0o755)
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
		"cli": {"kind": "cli", "paths": ["cmd/cli", "shared"]},
		"svc": {"kind": "service", "paths": ["cmd/svc", "shared"]}}}`)
	git("init", "-q", "-b", "main")
	commit("cmd/cli/main.go", "start the cli")
	commit("cmd/svc/main.go", "start the service")
	commit("shared/x.go", "share some code")
	commit("versions.json", "release cli v0.1.0")
	git("tag", "-a", "cli/v0.1.0", "-m", "First cli release.")
	commit("cmd/cli/main.go", "improve the cli")
	commit("cmd/svc/main.go", "improve the service")
	commit("versions.json", "release cli v0.2.0")
	git("tag", "-a", "cli/v0.2.0", "-m", "Second cli release.")

	tests := []struct{ component, tag, want string }{
		{"cli", "cli/v0.1.0", "First cli release.\n\n## Changes\n\n- start the cli (H)\n- share some code (H)\n"},
		{"cli", "cli/v0.2.0", "Second cli release.\n\n## Changes\n\n- improve the cli (H)\n"},
		{"svc", "", "Unreleased changes.\n\n## Changes\n\n- start the service (H)\n- share some code (H)\n- improve the service (H)\n"},
	}
	for _, tt := range tests {
		got, err := notes(tt.component, tt.tag)
		if err != nil {
			t.Fatal(err)
		}
		if got := hideHashes(got); got != tt.want {
			t.Errorf("%s %s:\n%s\nwant:\n%s", tt.component, tt.tag, got, tt.want)
		}
	}
}

func hideHashes(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if open := strings.LastIndex(l, " ("); open > 0 && strings.HasSuffix(l, ")") {
			lines[i] = l[:open] + " (H)"
		}
	}
	return strings.Join(lines, "\n")
}
