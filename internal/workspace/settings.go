package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

const settingsHeader = `# handcoding workspace settings
# language / locale: preferred solution and statement languages
# pack_settings: per-pack overrides; omitted fields inherit the workspace preference
# editor: empty uses VISUAL, then EDITOR, then vi
# theme: auto | dark | light; images: auto | kitty | blocks | none
# execution.backend: local (Docker is not implemented)
# go.command / go.debugger: executable names or workspace-relative paths
# python.interpreter: empty keeps hc's managed .hc/venv; a path selects an external Python
# agent.name: claude | codex; agent.command: [] uses that preset
# leetcode.site: com | cn
`

// PackSettings holds personal preferences, independently inherited when absent.
// These are not the languages/locales the pack declares in pack.yaml.
type PackSettings struct {
	Language string `yaml:"language,omitempty"`
	Locale   string `yaml:"locale,omitempty"`
}

func settingsDefaults(cfg Config, kind Kind) ([]byte, error) {
	n, err := yaml.ValueToNode(cfg)
	if err != nil {
		return nil, err
	}
	m := n.(*ast.MappingNode)
	if kind == Authoring {
		removeSetting(m, "packs")
	}
	return []byte(m.String() + "\n"), nil
}

// SyncSettings fills missing defaults and migrates local's manifest and old
// locale preferences. Existing settings win, including explicit empty values.
// It is independent of the release-only synchronization of agent skills.
func (w *Workspace) SyncSettings(locales map[string]string) error {
	return w.editSettings(func(m *ast.MappingNode) error {
		if err := w.ensureLocalManifest(); err != nil {
			return err
		}
		removeSetting(m, "locales")
		data, err := settingsDefaults(DefaultConfig(), w.Kind)
		if err != nil {
			return err
		}
		defaults, err := settingsMapping(data)
		if err != nil {
			return err
		}
		if err := fillSettings(m, defaults); err != nil {
			return err
		}
		names := make([]string, 0, len(locales))
		for name := range locales {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if name == "" || locales[name] == "" {
				continue
			}
			if err := checkSettingsPack(name); err != nil {
				return err
			}
			prefs, err := settingMap(m, "pack_settings", name)
			if err != nil {
				return err
			}
			if setting(prefs, "locale") == nil {
				if err := putSetting(prefs, "locale", locales[name]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// SavePackSetting changes one preference, re-reading under the settings lock
// so other hc processes' changes and unrelated YAML fields survive.
func (w *Workspace) SavePackSetting(name, key, value string) error {
	if err := checkSettingsPack(name); err != nil {
		return err
	}
	if key != "language" && key != "locale" {
		return fmt.Errorf("unknown pack setting %q", key)
	}
	return w.editSettings(func(m *ast.MappingNode) error {
		defaults, err := settingsDefaults(DefaultConfig(), w.Kind)
		if err != nil {
			return err
		}
		d, err := settingsMapping(defaults)
		if err != nil {
			return err
		}
		if err := fillSettings(m, d); err != nil {
			return err
		}
		prefs, err := settingMap(m, "pack_settings", name)
		if err != nil {
			return err
		}
		return putSetting(prefs, key, value)
	})
}

func checkSettingsPack(name string) error {
	if name == pack.LocalName {
		return nil
	}
	return pack.CheckName(name)
}

func (w *Workspace) editSettings(edit func(*ast.MappingNode) error) error {
	unlock, _, err := w.lock("settings.lock", true)
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(w.Root, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	missing := errors.Is(err, os.ErrNotExist)
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	// In particular, use the latest legacy locales when migrating.
	w.Config = cfg
	m, err := settingsMapping(data)
	if err != nil {
		return fmt.Errorf("%s: %w", ConfigFile, err)
	}
	before := m.String()
	if err := edit(m); err != nil {
		return fmt.Errorf("%s: %w", ConfigFile, err)
	}
	out := []byte(m.String() + "\n")
	if missing {
		out = append([]byte(settingsHeader), out...)
	}
	cfg = DefaultConfig()
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		return err
	}
	if before != m.String() || missing {
		if err := writeAtomic(path, out); err != nil {
			return err
		}
	}
	w.Config = cfg
	return nil
}

func settingsMapping(data []byte) (*ast.MappingNode, error) {
	f, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(f.Docs) > 1 {
		return nil, errors.New("must hold one YAML document")
	}
	if len(f.Docs) == 1 {
		if m, ok := f.Docs[0].Body.(*ast.MappingNode); ok {
			return m, nil
		}
		switch f.Docs[0].Body.(type) {
		case nil, *ast.CommentGroupNode:
		default:
			return nil, errors.New("must be a mapping")
		}
	}
	// Keep a comment-only file's header on the first inserted field.
	seed := append(bytes.TrimRight(data, "\n"), []byte("\nlanguage: go\n")...)
	f, err = parser.ParseBytes(seed, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return f.Docs[0].Body.(*ast.MappingNode), nil
}

func setting(m *ast.MappingNode, key string) *ast.MappingValueNode {
	for _, v := range m.Values {
		if s, ok := v.Key.(*ast.StringNode); ok && s.Value == key {
			return v
		}
	}
	return nil
}

func appendSetting(m *ast.MappingNode, v *ast.MappingValueNode) {
	column := m.GetToken().Position.Column
	if len(m.Values) > 0 {
		column = m.Values[0].Key.GetToken().Position.Column
	}
	v.AddColumn(column - v.Key.GetToken().Position.Column)
	m.Values = append(m.Values, v)
	if m.IsFlowStyle {
		m.SetIsFlowStyle(true)
	}
}

func putSetting(m *ast.MappingNode, key string, value any) error {
	n, err := yaml.ValueToNode(map[string]any{key: value})
	if err != nil {
		return err
	}
	entry := n.(*ast.MappingNode).Values[0]
	if v := setting(m, key); v != nil {
		if comment := v.Value.GetComment(); comment != nil {
			if err := entry.Value.SetComment(comment); err != nil {
				return err
			}
		}
		return v.Replace(entry.Value)
	}
	appendSetting(m, entry)
	return nil
}

func settingMap(m *ast.MappingNode, keys ...string) (*ast.MappingNode, error) {
	for _, key := range keys {
		if setting(m, key) == nil {
			if err := putSetting(m, key, map[string]any{}); err != nil {
				return nil, err
			}
		}
		v := setting(m, key)
		next, ok := v.Value.(*ast.MappingNode)
		if !ok {
			return nil, fmt.Errorf("%s must be a mapping", key)
		}
		// Empty {} becomes a regular block when it receives entries.
		if len(next.Values) == 0 {
			next.IsFlowStyle = false
			next.Start.Position.Column = v.Key.GetToken().Position.Column + 2
		}
		m = next
	}
	return m, nil
}

func fillSettings(m, defaults *ast.MappingNode) error {
	for _, d := range defaults.Values {
		key := d.Key.(*ast.StringNode).Value
		v := setting(m, key)
		if v == nil {
			appendSetting(m, d)
			continue
		}
		dm, nested := d.Value.(*ast.MappingNode)
		if !nested || len(dm.Values) == 0 {
			continue
		}
		vm, ok := v.Value.(*ast.MappingNode)
		if !ok {
			return fmt.Errorf("%s must be a mapping", key)
		}
		if len(vm.Values) == 0 {
			vm.Start.Position.Column = v.Key.GetToken().Position.Column + 2
		}
		if err := fillSettings(vm, dm); err != nil {
			return err
		}
	}
	return nil
}

func removeSetting(m *ast.MappingNode, key string) {
	for i, v := range m.Values {
		if s, ok := v.Key.(*ast.StringNode); !ok || s.Value != key {
			continue
		}
		comments := joinComments(v.GetComment(), v.Value.GetComment(), v.FootComment)
		m.Values = slices.Delete(m.Values, i, i+1)
		if comments != nil {
			switch {
			case i < len(m.Values):
				_ = m.Values[i].SetComment(joinComments(comments, m.Values[i].GetComment()))
			case len(m.Values) > 0:
				last := m.Values[len(m.Values)-1]
				last.FootComment = joinComments(last.FootComment, comments)
			default:
				_ = m.SetComment(joinComments(m.GetComment(), comments))
			}
		}
		return
	}
}

func joinComments(groups ...*ast.CommentGroupNode) *ast.CommentGroupNode {
	var comments []*ast.CommentNode
	for _, g := range groups {
		if g != nil {
			comments = append(comments, g.Comments...)
		}
	}
	if len(comments) == 0 {
		return nil
	}
	return &ast.CommentGroupNode{BaseNode: &ast.BaseNode{}, Comments: comments}
}

func (w *Workspace) defaultLocalManifest() *pack.Manifest {
	return &pack.Manifest{Protocol: problem.Protocol, Name: pack.LocalName, Locales: w.Config.LocalLocales(), Dir: w.PackPath(pack.LocalName)}
}

func (w *Workspace) ensureLocalManifest() error {
	if w.Kind != Practice {
		return nil
	}
	path := filepath.Join(w.PackPath(pack.LocalName), pack.ManifestFile)
	if _, err := os.Stat(path); err == nil {
		if w.Config.Locales != nil {
			m, err := pack.Load(filepath.Dir(path))
			if err != nil {
				return err
			}
			if !slices.Equal(m.Locales, w.Config.Locales) {
				return errors.New("locales conflicts with packs/local/pack.yaml; reconcile the declarations before removing locales from hc.yaml")
			}
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m := w.defaultLocalManifest()
	if err := m.ValidateLocal(); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, []byte(strings.TrimRight(string(data), "\n")+"\n"))
}
