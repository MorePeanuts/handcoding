package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/MorePeanuts/handcoding/internal/pack"
)

// UserGroup is a personal ordered list, independent of a pack's manifest
// and locales. Missing problems are retained when a pack changes.
type UserGroup struct {
	Name     string   `yaml:"name"`
	Title    string   `yaml:"title,omitempty"`
	Problems []string `yaml:"problems"`
}

type userGroupsFile struct {
	Groups []UserGroup `yaml:"groups"`
}

// CheckGroupName uses the pack name syntax, including local: it is a
// valid group name and the synthesized practice pack's name.
func CheckGroupName(name string) error {
	if name == pack.LocalName {
		return nil
	}
	return pack.CheckName(name)
}

// UserGroupsPath is a personal file, not a cache or part of an installed pack.
// Callers validate the pack name before using it.
func (w *Workspace) UserGroupsPath(name string) string {
	return filepath.Join(w.Root, "groups", name+".yaml")
}

// ReadUserGroups checks the file's structure, but not whether its members
// still exist. A bad personal file never makes the pack manifest invalid.
func (w *Workspace) ReadUserGroups(name string) ([]UserGroup, error) {
	if err := CheckGroupName(name); err != nil {
		return nil, err
	}
	path := w.UserGroupsPath(name)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f userGroupsFile
	dec := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", w.Rel(path), err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: must hold a single YAML document", w.Rel(path))
	}
	if err := checkUserGroups(f.Groups); err != nil {
		return nil, fmt.Errorf("%s: %w", w.Rel(path), err)
	}
	return f.Groups, nil
}

func checkUserGroups(groups []UserGroup) error {
	names := map[string]bool{}
	for _, g := range groups {
		if err := CheckGroupName(g.Name); err != nil {
			return fmt.Errorf("group %q: %w", g.Name, err)
		}
		if names[g.Name] {
			return fmt.Errorf("duplicate personal group %q", g.Name)
		}
		names[g.Name] = true
		seen := map[string]bool{}
		for _, slug := range g.Problems {
			if CheckGroupName(strings.ToLower(slug)) != nil {
				return fmt.Errorf("group %s: invalid problem slug %q", g.Name, slug)
			}
			if seen[slug] {
				return fmt.Errorf("group %s: duplicate problem %q", g.Name, slug)
			}
			seen[slug] = true
		}
	}
	return nil
}

// UpdateUserGroups serializes read-modify-write operations across hc
// processes and replaces the file atomically. A failed edit writes nothing.
func (w *Workspace) UpdateUserGroups(name string, edit func([]UserGroup) ([]UserGroup, error)) error {
	if err := CheckGroupName(name); err != nil {
		return err
	}
	unlock, _, err := w.lock("groups.lock", true)
	if err != nil {
		return err
	}
	defer unlock()
	groups, err := w.ReadUserGroups(name)
	if err != nil {
		return err
	}
	groups, err = edit(groups)
	if err != nil {
		return err
	}
	if err := checkUserGroups(groups); err != nil {
		return err
	}
	return w.writeUserGroups(name, groups)
}

func (w *Workspace) writeUserGroups(name string, groups []UserGroup) error {
	path := w.UserGroupsPath(name)
	if len(groups) == 0 {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	// Empty groups are lists too, rather than YAML null.
	groups = slices.Clone(groups)
	for i := range groups {
		if groups[i].Problems == nil {
			groups[i].Problems = []string{}
		}
	}
	data, err := yaml.MarshalWithOptions(userGroupsFile{Groups: groups}, yaml.IndentSequence(true))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// PurgeUserGroups also removes malformed files, without parsing them.
func (w *Workspace) PurgeUserGroups(name string) (bool, error) {
	if err := CheckGroupName(name); err != nil {
		return false, err
	}
	unlock, _, err := w.lock("groups.lock", true)
	if err != nil {
		return false, err
	}
	defer unlock()
	err = os.Remove(w.UserGroupsPath(name))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
