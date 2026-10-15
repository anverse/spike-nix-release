package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/anverse/spike-nix-release/tools/internal/project"
)

// manifestData is what the Homebrew and Scoop templates see. URL and
// SHA256 are keyed by platform, e.g. .URL.darwin_arm64.
type manifestData struct {
	Version  string
	Homepage string
	URL      map[string]string
	SHA256   map[string]string
}

// manifestsCommand renders every <component>.*.tmpl under -templates into
// -out, with the archives' URLs and checksums. The Nix build of a CLI
// release runs it, so the manifests always match the archives.
func manifestsCommand(args []string) error {
	fs := flag.NewFlagSet("manifests", flag.ExitOnError)
	component := fs.String("component", "", "component name")
	version := fs.String("version", "", "version")
	projectDir := fs.String("project", ".", "directory with project.json")
	archives := fs.String("archives", "", "directory with the archives and checksums.txt")
	templates := fs.String("templates", "", "directory with the templates")
	out := fs.String("out", "", "output directory")
	fs.Parse(args)

	p, err := project.Load(*projectDir)
	if err != nil {
		return err
	}
	data, err := readArchives(p, *component, *version, *archives)
	if err != nil {
		return err
	}
	return renderManifests(*component, *templates, *out, data)
}

// readArchives reads checksums.txt and derives each archive's platform from
// its name, <component>_<version>_<os>_<arch>.<ext>.
func readArchives(p project.Project, component, version, dir string) (manifestData, error) {
	tag := project.Tag(component, version)
	data := manifestData{
		Version:  version,
		Homepage: fmt.Sprintf("https://github.com/%s/%s", p.Owner, p.Repo),
		URL:      map[string]string{},
		SHA256:   map[string]string{},
	}
	f, err := os.Open(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		return data, err
	}
	defer f.Close()
	prefix := component + "_" + version + "_"
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		sum, file, ok := strings.Cut(lines.Text(), "  ")
		if !ok || !strings.HasPrefix(file, prefix) {
			continue
		}
		platform := strings.TrimPrefix(file, prefix)
		platform = strings.TrimSuffix(strings.TrimSuffix(platform, ".tar.gz"), ".zip")
		data.URL[platform] = fmt.Sprintf("%s/releases/download/%s/%s", data.Homepage, tag, file)
		data.SHA256[platform] = sum
	}
	return data, lines.Err()
}

func renderManifests(component, templates, out string, data manifestData) error {
	paths, err := filepath.Glob(filepath.Join(templates, "*", component+".*.tmpl"))
	if err != nil || len(paths) == 0 {
		return fmt.Errorf("no templates for %s in %s", component, templates)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, path := range paths {
		// A missing platform must fail the build, not leave an empty hash.
		tmpl, err := template.New(filepath.Base(path)).Option("missingkey=error").ParseFiles(path)
		if err != nil {
			return err
		}
		f, err := os.Create(filepath.Join(out, strings.TrimSuffix(filepath.Base(path), ".tmpl")))
		if err != nil {
			return err
		}
		err = tmpl.Execute(f, data)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
