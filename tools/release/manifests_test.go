package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderManifests(t *testing.T) {
	dir := t.TempDir()
	in := manifestsInput{
		Component: "tool",
		Version:   "1.2.3",
		Project:   filepath.Join(dir, "project.json"),
		Archives:  filepath.Join(dir, "archives"),
		Templates: filepath.Join(dir, "templates"),
		Out:       filepath.Join(dir, "out"),
	}
	write := func(path, text string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(in.Project, `{"owner": "o", "repo": "r"}`)
	write(filepath.Join(in.Archives, "checksums.txt"),
		"aaa  tool_1.2.3_windows_amd64.zip\nbbb  tool_1.2.3_linux_arm64.tar.gz\n")
	write(filepath.Join(in.Templates, "scoop", "tool.json.tmpl"),
		`{{.Version}} {{.URL.windows_amd64}} {{.SHA256.windows_amd64}} {{.SHA256.linux_arm64}}`)

	if err := renderManifests(in); err != nil {
		t.Fatal(err)
	}
	rendered, err := os.ReadFile(filepath.Join(in.Out, "tool.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3 https://github.com/o/r/releases/download/tool/v1.2.3/tool_1.2.3_windows_amd64.zip aaa bbb"
	if got := string(rendered); got != want {
		t.Errorf("tool.json=%s, want=%s", got, want)
	}

	// A platform without an archive fails instead of rendering an empty hash.
	write(filepath.Join(in.Templates, "scoop", "tool.json.tmpl"), `{{.SHA256.darwin_amd64}}`)
	err = renderManifests(in)
	if got, want := err != nil && strings.Contains(err.Error(), "darwin_amd64"), true; got != want {
		t.Errorf("error about the missing platform=%t, want=%t (err: %v)", got, want, err)
	}
}
