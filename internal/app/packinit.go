package app

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/builtin"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// The files hc pack init writes into a new pack, besides the skills,
// relative to its root.
const (
	WorkflowFile = ".github/workflows/check.yml"
	keepFile     = pack.ProblemsDir + "/.gitkeep"
	claudeFile   = "CLAUDE.md"
	ignoreFile   = ".gitignore"
)

// InitPackOptions configure InitPack.
type InitPackOptions struct {
	// Name is the pack's name; empty derives it from the directory's name
	// (PackNameFor).
	Name string
	// Title is the pack's display title; empty leaves it out of pack.yaml.
	Title string
	// HCVersion is the version of the running hc. CI installs it if it is
	// a release, and the latest release otherwise.
	HCVersion string
}

// InitPackResult is the pack InitPack created.
type InitPackResult struct {
	Root     string
	Manifest *pack.Manifest
	// Files are the files and skill directories written, relative to Root.
	Files []string
	// Pinned reports whether CI installs HCVersion rather than the latest
	// release.
	Pinned bool
	// GitRoot is the work tree Root is in, or "" when it is in none.
	GitRoot string
}

// InitPack scaffolds a pack repository in dir, created if missing, which then
// is an authoring workspace: pack.yaml, an empty problems/, a .gitignore for
// what hc adds to a checkout, a CI workflow running hc pack check, AGENTS.md
// and CLAUDE.md, and the authoring skills. dir must not be in a workspace,
// and none of these files may exist: InitPack checks them all before it
// writes any. It does not run git init.
func InitPack(dir string, o InitPackOptions) (*InitPackResult, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(root); err == nil && !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	if exists(filepath.Join(root, pack.ManifestFile)) {
		return nil, fmt.Errorf("%s already holds %s: it is a pack checkout, and hc works in it as it is", root, pack.ManifestFile)
	}
	if ws, ok := workspace.Lookup(root); ok {
		return nil, fmt.Errorf("%s is in the workspace %s: create the pack outside any workspace", root, ws)
	}
	m := &pack.Manifest{Protocol: problem.Protocol, Name: o.Name, Title: o.Title, Dir: root}
	if m.Name == "" {
		m.Name = PackNameFor(root)
		if err := pack.CheckName(m.Name); err != nil {
			return nil, fmt.Errorf("cannot name the pack after the directory %q (%w): give it a name with --name", filepath.Base(root), err)
		}
	} else if err := pack.CheckName(m.Name); err != nil {
		return nil, err
	}
	files, err := packFiles(m, o.HCVersion)
	if err != nil {
		return nil, err
	}
	skills := skill.Set(skill.Authoring)
	var all, taken []string
	for _, f := range files {
		all = append(all, f.path)
	}
	for _, d := range skill.Dirs {
		for _, name := range skills {
			all = append(all, path.Join(d, name))
		}
	}
	for _, p := range all {
		if exists(filepath.Join(root, filepath.FromSlash(p))) {
			taken = append(taken, p)
		}
	}
	if len(taken) > 0 {
		return nil, fmt.Errorf("%s already has files that hc pack init writes; move them away first:\n  %s", root, strings.Join(taken, "\n  "))
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, f.data, 0o644); err != nil {
			return nil, err
		}
	}
	// AGENTS.md has the author's section: the installer appends its block.
	if _, err := skill.Install(root, skills, skill.Options{Version: o.HCVersion, Exclusive: true}); err != nil {
		return nil, err
	}
	return &InitPackResult{
		Root:     root,
		Manifest: m,
		Files:    all,
		Pinned:   ReleaseVersion(o.HCVersion),
		GitRoot:  gitRoot(root),
	}, nil
}

type packFile struct {
	path string
	data []byte
}

// packFiles renders the files of a new pack with manifest m, whose CI
// installs hcVersion if it is a release.
func packFiles(m *pack.Manifest, hcVersion string) ([]packFile, error) {
	manifest, err := yaml.Marshal(struct {
		Protocol int          `yaml:"protocol"`
		Name     string       `yaml:"name"`
		Title    string       `yaml:"title,omitempty"`
		Groups   []pack.Group `yaml:"groups"`
	}{m.Protocol, m.Name, m.Title, []pack.Group{}})
	if err != nil {
		return nil, err
	}
	header := "# The manifest of this hc pack: `hc docs protocol packyaml` describes its fields.\n"
	data := struct {
		Name, Title, Version string
		Pinned               bool
	}{m.Name, m.DisplayTitle(), "latest", false}
	if ReleaseVersion(hcVersion) {
		data.Version, data.Pinned = hcVersion, true
	}
	files := []packFile{
		{pack.ManifestFile, append([]byte(header), manifest...)},
		{keepFile, nil},
	}
	for _, t := range []struct{ path, template string }{
		{ignoreFile, "gitignore"},
		{WorkflowFile, "check.yml"},
		{skill.AgentsFile, "AGENTS.md"},
		{claudeFile, "CLAUDE.md"},
	} {
		out, err := renderPackTemplate(t.template, data)
		if err != nil {
			return nil, err
		}
		files = append(files, packFile{t.path, out})
	}
	return files, nil
}

// renderPackTemplate renders an embedded template of builtin.PackInitFS.
// Its delimiters are <% %>, since the workflow may hold GitHub's ${{ }}.
func renderPackTemplate(name string, data any) ([]byte, error) {
	src, err := fs.ReadFile(builtin.PackInitFS(), name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Delims("<%", "%>").Option("missingkey=error").Parse(string(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	nonAlnumRE = regexp.MustCompile(`[^a-z0-9]+`)
	// releaseRE matches the tag of an hc release, vX.Y.Z or vX.Y.Z-pre.
	releaseRE = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
	// buildRE matches versions that look like releases but are not: git
	// describe's (v0.1.0-3-gabc1234, -dirty) and Go's pseudo-versions
	// (v0.0.0-20261028123456-abcdef123456).
	buildRE = regexp.MustCompile(`-\d+-g[0-9a-f]+(-dirty)?$|-dirty$|\d{14}-[0-9a-f]{12}$`)
)

// PackNameFor derives a pack's name from the name of its directory: lower
// case, with every run of other characters than letters and digits a hyphen.
// The result may still not be a valid name (it is empty, say, or local).
func PackNameFor(dir string) string {
	return strings.Trim(nonAlnumRE.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "-"), "-")
}

// ReleaseVersion reports whether v is the version of an hc release, which
// `go install …@v` can install: not dev, nor a build between releases.
func ReleaseVersion(v string) bool {
	return releaseRE.MatchString(v) && !buildRE.MatchString(v)
}

// gitRoot returns the top of the git work tree dir is in, or "".
func gitRoot(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if exists(filepath.Join(d, ".git")) {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}
