package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/anverse/spike-nix-release/tools/internal/repo"
)

// manifestsInput is what `release manifests` reads.
type manifestsInput struct {
	Component, Version string
	Project            string // project.json
	Archives           string // directory with the archives and checksums.txt
	Templates          string // directory with <tool>/<component>.*.tmpl
	Out                string // output directory
}

// manifestData is what the templates see. URL and SHA256 are keyed by
// platform: {{.URL.darwin_arm64}}, {{.SHA256.darwin_arm64}}.
type manifestData struct {
	Version  string
	Homepage string
	URL      map[string]string
	SHA256   map[string]string
}

// renderManifests renders every <component>.*.tmpl into the output
// directory, with the URLs and checksums of the archives. The Nix build of
// a CLI release runs it, so the manifests always match the archives.
func renderManifests(in manifestsInput) error {
	project, err := repo.LoadProjectFile(in.Project)
	if err != nil {
		return err
	}
	data, err := readArchives(project, in.Component, in.Version, in.Archives)
	if err != nil {
		return err
	}
	templates, err := filepath.Glob(filepath.Join(in.Templates, "*", in.Component+".*.tmpl"))
	if err != nil || len(templates) == 0 {
		return fmt.Errorf("no templates for %s in %s", in.Component, in.Templates)
	}
	if err := os.MkdirAll(in.Out, 0o755); err != nil {
		return err
	}
	for _, t := range templates {
		out := filepath.Join(in.Out, strings.TrimSuffix(filepath.Base(t), ".tmpl"))
		if err := render(t, out, data); err != nil {
			return err
		}
	}
	return nil
}

// readArchives reads checksums.txt. Each archive's platform comes from its
// name, <component>_<version>_<os>_<arch>.<extension>.
func readArchives(project repo.Project, component, version, dir string) (manifestData, error) {
	data := manifestData{
		Version:  version,
		Homepage: fmt.Sprintf("https://github.com/%s/%s", project.Owner, project.Repo),
		URL:      map[string]string{},
		SHA256:   map[string]string{},
	}
	downloads := data.Homepage + "/releases/download/" + repo.Tag(component, version)
	prefix := component + "_" + version + "_"

	f, err := os.Open(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		return data, err
	}
	defer f.Close()
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		checksum, file, ok := strings.Cut(lines.Text(), "  ")
		if !ok || !strings.HasPrefix(file, prefix) {
			continue
		}
		platform := strings.TrimPrefix(file, prefix)
		platform = strings.TrimSuffix(strings.TrimSuffix(platform, ".tar.gz"), ".zip")
		data.URL[platform] = downloads + "/" + file
		data.SHA256[platform] = checksum
	}
	return data, lines.Err()
}

func render(templateFile, outFile string, data manifestData) error {
	// A platform without an archive fails, instead of rendering an empty hash.
	t, err := template.New(filepath.Base(templateFile)).Option("missingkey=error").ParseFiles(templateFile)
	if err != nil {
		return err
	}
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, data)
}
