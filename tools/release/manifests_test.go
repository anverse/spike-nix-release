package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

func TestManifests(t *testing.T) {
	dir := t.TempDir()
	archives := filepath.Join(dir, "archives")
	templates := filepath.Join(dir, "templates")
	out := filepath.Join(dir, "out")
	os.MkdirAll(archives, 0o755)
	os.MkdirAll(filepath.Join(templates, "scoop"), 0o755)
	os.WriteFile(filepath.Join(archives, "checksums.txt"), []byte(
		"aaa  tool_1.2.3_windows_amd64.zip\nbbb  tool_1.2.3_linux_arm64.tar.gz\n"), 0o644)
	os.WriteFile(filepath.Join(templates, "scoop", "tool.json.tmpl"), []byte(
		`{{.Version}} {{.URL.windows_amd64}} {{.SHA256.windows_amd64}} {{.SHA256.linux_arm64}}`), 0o644)

	p := project.Project{Owner: "o", Repo: "r"}
	data, err := readArchives(p, "tool", "1.2.3", archives)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderManifests("tool", templates, out, data); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(out, "tool.json"))
	want := "1.2.3 https://github.com/o/r/releases/download/tool/v1.2.3/tool_1.2.3_windows_amd64.zip aaa bbb"
	if string(got) != want {
		t.Errorf("got %s\nwant %s", got, want)
	}

	// A platform without an archive fails instead of rendering an empty hash.
	os.WriteFile(filepath.Join(templates, "scoop", "tool.json.tmpl"), []byte(`{{.SHA256.darwin_amd64}}`), 0o644)
	if err := renderManifests("tool", templates, out, data); err == nil || !strings.Contains(err.Error(), "darwin_amd64") {
		t.Errorf("want an error about the missing platform, got %v", err)
	}
}
