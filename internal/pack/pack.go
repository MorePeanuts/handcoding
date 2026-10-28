// Package pack defines a pack of problems and its manifest, pack.yaml
// (docs/protocol.md, "Packs").
//
// A pack is a directory, usually a repository of its own:
//
//	<dir>/
//	  pack.yaml        the manifest
//	  problems/<dir>/  one directory per problem (see package problem)
//
// Anything else in it (README, LICENSE, CI configuration, agent files) is not
// part of the pack and is ignored.
package pack

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/version"
)

const (
	// ManifestFile is the pack's manifest, at the root of the pack.
	ManifestFile = "pack.yaml"
	// ProblemsDir holds the pack's problems, one directory each.
	ProblemsDir = "problems"
	// LocalName is reserved: no pack may be called local, which is the name
	// of a workspace's own problems.
	LocalName = "local"
)

// Manifest is the content of pack.yaml.
type Manifest struct {
	// Protocol is the protocol version the pack is written against.
	Protocol int `yaml:"protocol"`
	// Name is the pack's identity. Installed packs, solutions and history
	// are keyed by it, so renaming a pack is a breaking change.
	Name string `yaml:"name"`
	// Title is the display name; see DisplayTitle.
	Title string `yaml:"title,omitempty"`
	// Version is the pack's own SemVer version, without a "v" prefix. It is
	// informational: installs are pinned by commit.
	Version     string   `yaml:"version,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Authors     []string `yaml:"authors,omitempty"`
	// License is an SPDX license expression.
	License  string `yaml:"license,omitempty"`
	Homepage string `yaml:"homepage,omitempty"`
	// Languages are the protocol language ids the pack's problems can be
	// solved in. Empty means every language of Protocol; see
	// SolutionLanguages.
	Languages []string `yaml:"languages,omitempty"`
	// Groups are ordered, named lists of the pack's problems.
	Groups []Group `yaml:"groups,omitempty"`

	// Dir is the directory the pack was loaded from.
	Dir string `yaml:"-"`
}

// Group is a named, ordered list of problems of the pack, by slug. A problem
// may be in several groups or in none.
type Group struct {
	Name     string   `yaml:"name"`
	Title    string   `yaml:"title,omitempty"`
	Problems []string `yaml:"problems,omitempty"`
}

// nameRE is the syntax of pack and group names: lower-case kebab-case.
var nameRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// semverRE is SemVer 2.0.0 (semver.org), without a "v" prefix.
var semverRE = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// ProtocolError reports a pack whose protocol version hc does not read: newer
// than problem.Protocol, or older than problem.MinProtocol.
type ProtocolError struct {
	// Protocol is the version the pack declares.
	Protocol int
}

func (e *ProtocolError) Error() string {
	if e.Protocol > problem.Protocol {
		return fmt.Sprintf("this pack needs protocol %d, hc %s supports up to %d, upgrade hc", e.Protocol, version.String(), problem.Protocol)
	}
	return fmt.Sprintf("this pack uses protocol %d, which hc %s no longer reads, update the pack", e.Protocol, version.String())
}

// checkProtocol checks that hc reads protocol version n.
func checkProtocol(n int) error {
	switch {
	case n <= 0:
		return fmt.Errorf("protocol %d must be a positive integer", n)
	case n < problem.MinProtocol || n > problem.Protocol:
		return &ProtocolError{Protocol: n}
	}
	return nil
}

// Load reads the manifest of the pack in dir. It rejects unknown fields, and
// a protocol version hc does not read, before anything else, since a newer
// protocol may add fields this hc does not know. The rest of the manifest is
// checked by Validate.
func Load(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return nil, err
	}
	// Errors name the file pack.yaml; callers say where the pack is.
	name := ManifestFile
	// The YAML decoder would turn "1" and 1.5 into the int 1, so the type
	// of protocol is checked on its own.
	var head struct {
		Protocol any `yaml:"protocol"`
	}
	if yaml.Unmarshal(data, &head) == nil {
		var err error
		switch v := head.Protocol.(type) {
		case nil:
			// Missing: Validate reports it.
		case uint64:
			err = checkProtocol(int(min(v, math.MaxInt32)))
		case int64:
			err = checkProtocol(int(max(min(v, math.MaxInt32), math.MinInt32)))
		case string:
			err = fmt.Errorf("protocol must be an integer, such as %d, not the string %q", problem.Protocol, v)
		default:
			err = fmt.Errorf("protocol must be an integer, such as %d, not %v", problem.Protocol, v)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	var m Manifest
	if err := yaml.UnmarshalWithOptions(data, &m, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	m.Dir = dir
	return &m, nil
}

// ProblemsPath is the directory holding the pack's problems.
func (m *Manifest) ProblemsPath() string { return filepath.Join(m.Dir, ProblemsDir) }

// DisplayTitle is the title, or the name when there is none.
func (m *Manifest) DisplayTitle() string {
	if m.Title != "" {
		return m.Title
	}
	return m.Name
}

// DisplayTitle is the group's title, or its name when there is none.
func (g *Group) DisplayTitle() string {
	if g.Title != "" {
		return g.Title
	}
	return g.Name
}

// SolutionLanguages are the languages the pack's problems can be solved in:
// languages when given, else every language of the pack's protocol.
func (m *Manifest) SolutionLanguages() []string {
	if len(m.Languages) > 0 {
		return m.Languages
	}
	return problem.Languages(m.Protocol)
}

// CheckName checks a pack's name: required, lower-case kebab-case, and not
// local.
func CheckName(name string) error {
	switch {
	case name == "":
		return errors.New("name is required")
	case !nameRE.MatchString(name):
		return fmt.Errorf("name %q must be lower-case kebab-case: lower-case letters, digits and single hyphens", name)
	case name == LocalName:
		return fmt.Errorf("name %s is reserved for a workspace's own problems", LocalName)
	}
	return nil
}

// Copy copies the pack in src, its pack.yaml and problems/ but nothing else,
// to dst, which must not hold them yet.
func Copy(src, dst string) error {
	data, err := os.ReadFile(filepath.Join(src, ManifestFile))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	problems := filepath.Join(src, ProblemsDir)
	if fi, err := os.Stat(problems); err == nil && fi.IsDir() {
		if err := os.CopyFS(filepath.Join(dst, ProblemsDir), os.DirFS(problems)); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dst, ManifestFile), data, 0o644)
}

// Validate checks the manifest on its own. ValidateProblems checks it
// against the pack's problems.
func (m *Manifest) Validate() error {
	var errs []error
	// A missing protocol decodes as 0.
	protoErr := checkProtocol(m.Protocol)
	if m.Protocol == 0 {
		protoErr = fmt.Errorf("protocol is required: the protocol version the pack is written against, %d for this hc", problem.Protocol)
	}
	if protoErr != nil {
		errs = append(errs, protoErr)
	}
	if err := CheckName(m.Name); err != nil {
		errs = append(errs, err)
	}
	if m.Version != "" && !semverRE.MatchString(m.Version) {
		hint := ""
		if semverRE.MatchString(m.Version[1:]) && (m.Version[0] == 'v' || m.Version[0] == 'V') {
			hint = fmt.Sprintf(" (write %s, without the v)", m.Version[1:])
		}
		errs = append(errs, fmt.Errorf("version %q must be a SemVer version such as 1.2.0%s", m.Version, hint))
	}
	if m.Languages != nil && len(m.Languages) == 0 {
		errs = append(errs, errors.New("languages must not be empty; leave it out for every language of the protocol"))
	}
	// An unknown protocol has no languages to check against.
	if protoErr == nil {
		known := problem.Languages(m.Protocol)
		seen := map[string]bool{}
		for _, l := range m.Languages {
			switch {
			case seen[l]:
				errs = append(errs, fmt.Errorf("languages: duplicate language %q", l))
			case !slices.Contains(known, l):
				errs = append(errs, fmt.Errorf("languages: unknown language %q (protocol %d has %s)", l, m.Protocol, joinList(known)))
			}
			seen[l] = true
		}
	}
	groups := map[string]bool{}
	for i, g := range m.Groups {
		switch {
		case g.Name == "":
			errs = append(errs, fmt.Errorf("groups[%d]: name is required", i))
		case !nameRE.MatchString(g.Name):
			errs = append(errs, fmt.Errorf("group %q: name must be lower-case kebab-case: lower-case letters, digits and single hyphens", g.Name))
		case groups[g.Name]:
			errs = append(errs, fmt.Errorf("group %s: duplicate group name", g.Name))
		}
		groups[g.Name] = true
		seen := map[string]bool{}
		for _, slug := range g.Problems {
			if seen[slug] {
				errs = append(errs, fmt.Errorf("group %s: problem %s is listed more than once", g.Name, slug))
			}
			seen[slug] = true
		}
	}
	return errors.Join(errs...)
}

// ValidateProblems checks the manifest against the pack's problems: every
// group entry must be the slug of one of them.
func (m *Manifest) ValidateProblems(probs []*problem.Problem) error {
	slugs := map[string]bool{}
	for _, p := range probs {
		slugs[p.Slug] = true
	}
	var errs []error
	for _, g := range m.Groups {
		for _, slug := range g.Problems {
			if !slugs[slug] {
				errs = append(errs, fmt.Errorf("group %s: %s is not a problem of the pack", g.Name, slug))
			}
		}
	}
	return errors.Join(errs...)
}

// joinList writes "a", "a and b" or "a, b and c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return "none"
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
