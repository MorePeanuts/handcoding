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
)

// packsKey is the key of Config.Packs in hc.yaml.
const packsKey = "packs"

// checkPacks checks the packs list of hc.yaml, data: a practice workspace's
// entries must name distinct packs and their sources, and an authoring
// workspace has no installed packs, so no list.
func checkPacks(data []byte, kind Kind, list []PackSource) error {
	if kind == Authoring {
		var keys map[string]any
		if yaml.Unmarshal(data, &keys) == nil {
			if _, ok := keys[packsKey]; ok {
				return errors.New("packs: lists the packs installed in a practice workspace, and a pack checkout has none: remove it from this hc.yaml")
			}
		}
		return nil
	}
	var errs []error
	seen := map[string]bool{}
	for i, s := range list {
		if err := pack.CheckName(s.Name); err != nil {
			errs = append(errs, fmt.Errorf("packs[%d]: %w", i, err))
		} else if seen[s.Name] {
			errs = append(errs, fmt.Errorf("packs[%d]: pack %s is listed more than once", i, s.Name))
		}
		seen[s.Name] = true
		if s.Source == "" {
			errs = append(errs, fmt.Errorf("packs[%d]: source is required", i))
		}
	}
	return errors.Join(errs...)
}

// PacksPath is the directory of the packs, packs/ (in a practice workspace).
func (w *Workspace) PacksPath() string { return filepath.Join(w.Root, PacksDir) }

// PackPath is the directory of the pack called name in packs/.
func (w *Workspace) PackPath(name string) string { return filepath.Join(w.PacksPath(), name) }

// SavePacks makes list the packs list of hc.yaml, and of w.Config. Only that
// list is rewritten: the rest of the file keeps its comments, order and
// formatting. An empty list removes the key.
func (w *Workspace) SavePacks(list []PackSource) error {
	if w.Kind != Practice {
		return errors.New("a pack checkout has no installed packs")
	}
	path := filepath.Join(w.Root, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := setPacks(data, list)
	if err != nil {
		return fmt.Errorf("%s: %w", ConfigFile, err)
	}
	if err := writeAtomic(path, out); err != nil {
		return err
	}
	w.Config.Packs = list
	return nil
}

// setPacks returns the YAML document data with list as its packs value.
func setPacks(data []byte, list []PackSource) ([]byte, error) {
	f, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(f.Docs) > 1 {
		return nil, errors.New("holds several YAML documents")
	}
	var m *ast.MappingNode
	if len(f.Docs) == 1 {
		switch body := f.Docs[0].Body.(type) {
		case *ast.MappingNode:
			m = body
		case nil, *ast.CommentGroupNode:
			// Nothing but comments: the list goes after them.
		default:
			return nil, fmt.Errorf("must be a mapping, not %s", body.Type())
		}
	}
	var entry *ast.MappingValueNode
	if len(list) > 0 {
		n, err := yaml.ValueToNode(struct {
			Packs []PackSource `yaml:"packs"`
		}{list}, yaml.IndentSequence(true))
		if err != nil {
			return nil, err
		}
		entry = n.(*ast.MappingNode).Values[0]
	}
	if m == nil {
		if entry == nil {
			return data, nil
		}
		text := entry.String() + "\n"
		if len(bytes.TrimSpace(data)) > 0 {
			text = string(bytes.TrimRight(data, "\n")) + "\n" + text
		}
		return []byte(text), nil
	}
	values := m.Values[:0:0]
	done := false
	// The comment above a removed packs key, which may be the file's
	// header: it goes to the key after it.
	var orphan *ast.CommentGroupNode
	for _, v := range m.Values {
		if v.Key.String() != packsKey {
			if orphan != nil && v.GetComment() == nil {
				if err := v.SetComment(orphan); err != nil {
					return nil, err
				}
			}
			orphan = nil
			values = append(values, v)
			continue
		}
		if entry != nil && !done {
			if err := v.Replace(entry.Value); err != nil {
				return nil, err
			}
			values = append(values, v)
			done = true
			continue
		}
		done = true
		orphan = v.GetComment()
		// The comment after it, such as one at the end of the file, goes to
		// the key before it, or else to the key after it.
		if foot := v.FootComment; foot != nil {
			switch {
			case len(values) > 0 && values[len(values)-1].FootComment == nil:
				values[len(values)-1].FootComment = foot
			case orphan == nil:
				orphan = foot
			default:
				orphan = &ast.CommentGroupNode{BaseNode: &ast.BaseNode{}, Comments: append(slices.Clone(orphan.Comments), foot.Comments...)}
			}
		}
	}
	if entry != nil && !done {
		values = append(values, entry)
	}
	if len(values) == 0 {
		// Nothing is left but, perhaps, the comment.
		if orphan != nil {
			return []byte(orphan.String() + "\n"), nil
		}
		return nil, nil
	}
	m.Values = values
	if m.IsFlowStyle {
		m.SetIsFlowStyle(true)
	}
	return []byte(strings.TrimRight(f.String(), "\n") + "\n"), nil
}

// writeAtomic replaces the file at path with data, keeping its mode.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
