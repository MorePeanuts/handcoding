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

// localesKey is the key of Config.Locales in hc.yaml.
const localesKey = "locales"

// checkLocales checks the locales of hc.yaml, data, which are the local
// pack's: a pack checkout has no local pack, and its locales are in its
// pack.yaml.
func checkLocales(data []byte, kind Kind, locales []string) error {
	if kind == Authoring {
		var keys map[string]any
		if yaml.Unmarshal(data, &keys) == nil {
			if _, ok := keys[localesKey]; ok {
				return errors.New("locales: are the locales of a practice workspace's own problems, and a pack checkout's are in its pack.yaml: remove it from this hc.yaml")
			}
		}
		return nil
	}
	if locales == nil {
		return nil
	}
	return problem.CheckLocales(locales)
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

// RemoveFromGroups removes slug from every group of the pack's pack.yaml,
// keeping its comments and layout, and returns the names of the groups it
// was in. The local pack has no pack.yaml, and no groups.
func (k *Pack) RemoveFromGroups(slug string) ([]string, error) {
	path := filepath.Join(k.Dir, pack.ManifestFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && k.Name == pack.LocalName {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out, groups, err := removeFromGroups(data, slug)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pack.ManifestFile, err)
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return groups, writeAtomic(path, out)
}

// removeFromGroups returns the pack.yaml document data without slug in the
// problems of any group, and the names of the groups that had it.
func removeFromGroups(data []byte, slug string) ([]byte, []string, error) {
	f, err := parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	if len(f.Docs) != 1 {
		return nil, nil, errors.New("must hold one YAML document")
	}
	m, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, nil, errors.New("must be a mapping")
	}
	var groups []string
	for _, v := range m.Values {
		seq, ok := v.Value.(*ast.SequenceNode)
		if v.Key.String() != "groups" || !ok {
			continue
		}
		for _, g := range seq.Values {
			gm, ok := g.(*ast.MappingNode)
			if !ok {
				continue
			}
			name := ""
			for _, gv := range gm.Values {
				if gv.Key.String() == "name" {
					name = gv.Value.String()
				}
			}
			for _, gv := range gm.Values {
				probs, ok := gv.Value.(*ast.SequenceNode)
				if !ok || gv.Key.String() != "problems" || !dropValue(probs, slug) {
					continue
				}
				groups = append(groups, name)
				if len(probs.Values) == 0 && !probs.IsFlowStyle {
					// An empty block sequence would print as [] on a line
					// of its own.
					empty, err := yaml.ValueToNode([]string{}, yaml.Flow(true))
					if err != nil {
						return nil, nil, err
					}
					if err := gv.Replace(empty); err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}
	if len(groups) == 0 {
		return data, nil, nil
	}
	return []byte(strings.TrimRight(f.String(), "\n") + "\n"), groups, nil
}

// dropValue removes the scalar entries equal to value from seq, with their
// comments, and reports whether there were any.
func dropValue(seq *ast.SequenceNode, value string) bool {
	comments := len(seq.ValueHeadComments) == len(seq.Values)
	var keep []ast.Node
	var keepComments []*ast.CommentGroupNode
	for i, e := range seq.Values {
		if s, ok := e.(*ast.StringNode); ok && s.Value == value {
			continue
		}
		keep = append(keep, e)
		if comments {
			keepComments = append(keepComments, seq.ValueHeadComments[i])
		}
	}
	if len(keep) == len(seq.Values) {
		return false
	}
	seq.Values = keep
	if comments {
		seq.ValueHeadComments = keepComments
	}
	return true
}
