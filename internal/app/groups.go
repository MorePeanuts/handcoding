package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// Group is a browsing list with an explicit owner. Personal groups never
// become part of the pack manifest or its publishing checks.
type Group struct {
	pack.Group
	Pack string
	User bool
}

// Key distinguishes equal names from different sources.
func (g Group) Key() string {
	if g.User {
		return "user:" + g.Name
	}
	return "pack:" + g.Name
}

// Select returns existing members in order, and the retained slugs that
// cannot currently be loaded. Progress counts only existing members.
func (g Group) Select(all []*problem.Problem) (probs []*problem.Problem, missing []string) {
	bySlug := map[string]*problem.Problem{}
	for _, p := range all {
		if p.Pack == g.Pack {
			bySlug[p.Slug] = p
		}
	}
	for _, slug := range g.Problems {
		if p := bySlug[slug]; p != nil {
			probs = append(probs, p)
		} else {
			missing = append(missing, slug)
		}
	}
	return
}

// Groups returns personal groups followed by the pack's groups. A broken
// personal file returns the pack groups alongside its error, so browsing
// the pack remains possible.
func (a *App) Groups(name string) ([]Group, error) {
	k, err := a.WS.Pack(name)
	if err != nil {
		return nil, err
	}
	user, userErr := a.WS.ReadUserGroups(name)
	var groups []Group
	for _, g := range user {
		groups = append(groups, Group{Group: pack.Group{Name: g.Name, Title: g.Title, Problems: g.Problems}, Pack: name, User: true})
	}
	return append(groups, packGroups(k)...), userErr
}

func packGroups(k *workspace.Pack) []Group {
	var groups []Group
	for _, g := range k.Manifest.Groups {
		groups = append(groups, Group{Group: g, Pack: k.Name})
	}
	return groups
}

// ResolveGroup requires a unique match across sources and, without a
// pack name, packs. Explicit pack: selectors work even if personal data is
// malformed; an unqualified name cannot safely resolve in that situation.
func (a *App) ResolveGroup(packName, selector string) (Group, error) {
	source, name := "", selector
	if prefix, rest, ok := strings.Cut(selector, ":"); ok {
		if prefix != "user" && prefix != "pack" {
			return Group{}, fmt.Errorf("unknown group source %q: use user:<name> or pack:<name>", prefix)
		}
		source, name = prefix, rest
	}
	if err := workspace.CheckGroupName(name); err != nil {
		return Group{}, err
	}
	names := []string{packName}
	if packName == "" {
		packs, err := a.WS.Packs()
		if err != nil {
			return Group{}, err
		}
		names = nil
		for _, k := range packs {
			if k.Err == nil {
				names = append(names, k.Name)
			}
		}
	}
	var matches []Group
	var errs []error
	for _, n := range names {
		var gs []Group
		var err error
		if source == "pack" {
			var k *workspace.Pack
			k, err = a.WS.Pack(n)
			if err == nil {
				gs = packGroups(k)
			}
		} else {
			gs, err = a.Groups(n)
		}
		if err != nil {
			errs = append(errs, err)
		}
		for _, g := range gs {
			if g.Name == name && (source == "" || (source == "user") == g.User) {
				matches = append(matches, g)
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return Group{}, err
	}
	switch len(matches) {
	case 0:
		return Group{}, fmt.Errorf("group %q not found; use hc group list%s", selector, packFlag(packName))
	case 1:
		return matches[0], nil
	default:
		var choices []string
		for _, g := range matches {
			choices = append(choices, "--pack "+g.Pack+" --group "+g.Key())
		}
		return Group{}, fmt.Errorf("group %q is ambiguous; choose %s", selector, strings.Join(choices, " or "))
	}
}

func packFlag(name string) string {
	if name == "" {
		return ""
	}
	return " --pack " + name
}

// FilterProblems shares CLI and TUI group resolution without changing
// Workspace.Filter's pack-manifest-only semantics.
func (a *App) FilterProblems(all []*problem.Problem, name, selector string) ([]*problem.Problem, []string, error) {
	if selector == "" {
		probs, err := a.WS.Filter(all, name, "")
		return probs, nil, err
	}
	g, err := a.ResolveGroup(name, selector)
	if err != nil {
		return nil, nil, err
	}
	probs, missing := g.Select(all)
	return probs, missing, nil
}

// CreateGroup makes an empty personal group, even in an authoring workspace.
func (a *App) CreateGroup(packName, name, title string) error {
	if _, err := a.WS.Pack(packName); err != nil {
		return err
	}
	return a.WS.UpdateUserGroups(packName, func(gs []workspace.UserGroup) ([]workspace.UserGroup, error) {
		return append(gs, workspace.UserGroup{Name: name, Title: title}), nil
	})
}

// CopyGroup takes a snapshot of either source, including retained missing
// members. Future upstream edits cannot change it.
func (a *App) CopyGroup(packName, source, name, title string) error {
	g, err := a.ResolveGroup(packName, source)
	if err != nil {
		return err
	}
	if title == "" {
		title = g.Title
	}
	return a.WS.UpdateUserGroups(g.Pack, func(gs []workspace.UserGroup) ([]workspace.UserGroup, error) {
		return append(gs, workspace.UserGroup{Name: name, Title: title, Problems: slices.Clone(g.Problems)}), nil
	})
}

// editGroup resolves the source before writing and rechecks existence
// inside the lock, so concurrent edits do not overwrite one another.
func (a *App) editGroup(packName, selector string, edit func(*workspace.UserGroup) (bool, error)) error {
	g, err := a.ResolveGroup(packName, selector)
	if err != nil {
		return err
	}
	if !g.User {
		return fmt.Errorf("%s is a pack group; copy it to a personal group with hc group copy first", g.Key())
	}
	return a.WS.UpdateUserGroups(g.Pack, func(gs []workspace.UserGroup) ([]workspace.UserGroup, error) {
		i := slices.IndexFunc(gs, func(u workspace.UserGroup) bool { return u.Name == g.Name })
		if i < 0 {
			return nil, fmt.Errorf("personal group %q no longer exists", g.Name)
		}
		drop, err := edit(&gs[i])
		if err != nil {
			return nil, err
		}
		if drop {
			gs = slices.Delete(gs, i, i+1)
		}
		return gs, nil
	})
}

// RenameGroup can change the name and, when non-nil, its display title.
func (a *App) RenameGroup(packName, selector, name string, title *string) error {
	return a.editGroup(packName, selector, func(g *workspace.UserGroup) (bool, error) {
		g.Name = name
		if title != nil {
			g.Title = *title
		}
		return false, nil
	})
}

func (a *App) DeleteGroup(packName, selector string) error {
	return a.editGroup(packName, selector, func(*workspace.UserGroup) (bool, error) { return true, nil })
}

// AddGroupProblems resolves user-entered IDs/slugs within the group's pack
// to stable slugs. Re-adding a member is an idempotent operation.
func (a *App) AddGroupProblems(packName, selector string, refs []string) error {
	g, err := a.ResolveGroup(packName, selector)
	if err != nil {
		return err
	}
	var slugs []string
	for _, ref := range refs {
		if p, slug, ok := strings.Cut(ref, "/"); ok {
			if p != g.Pack {
				return fmt.Errorf("%s belongs to another pack; group %s belongs to %s", ref, g.Key(), g.Pack)
			}
			ref = slug
		}
		p, err := a.WS.Problem(g.Pack + "/" + ref)
		if err != nil {
			return err
		}
		slugs = append(slugs, p.Slug)
	}
	return a.editGroup(g.Pack, g.Key(), func(u *workspace.UserGroup) (bool, error) {
		for _, slug := range slugs {
			if !slices.Contains(u.Problems, slug) {
				u.Problems = append(u.Problems, slug)
			}
		}
		return false, nil
	})
}

// RemoveGroupProblems accepts stored slugs, including unavailable members.
func (a *App) RemoveGroupProblems(packName, selector string, slugs []string) error {
	return a.editGroup(packName, selector, func(g *workspace.UserGroup) (bool, error) {
		for _, slug := range slugs {
			if !slices.Contains(g.Problems, slug) {
				return false, fmt.Errorf("%s is not in group %s; give its stored slug", slug, g.Name)
			}
		}
		g.Problems = slices.DeleteFunc(g.Problems, func(s string) bool { return slices.Contains(slugs, s) })
		return false, nil
	})
}

// OrderGroupProblems requires every stored slug exactly once. It cannot
// accidentally drop a member or turn a missing reference into another one.
func (a *App) OrderGroupProblems(packName, selector string, slugs []string) error {
	return a.editGroup(packName, selector, func(g *workspace.UserGroup) (bool, error) {
		before, after := slices.Clone(g.Problems), slices.Clone(slugs)
		slices.Sort(before)
		slices.Sort(after)
		if !slices.Equal(before, after) {
			return false, errors.New("give every group member's stored slug exactly once, in the desired order")
		}
		g.Problems = slices.Clone(slugs)
		return false, nil
	})
}

// SaveGroup saves a complete personal group from the TUI form. Existing
// unavailable slugs may be retained; newly added ones must resolve exactly.
// The whole edit, including a rename, is committed in one write.
func (a *App) SaveGroup(packName, original string, group workspace.UserGroup) error {
	if _, err := a.WS.Pack(packName); err != nil {
		return err
	}
	return a.WS.UpdateUserGroups(packName, func(gs []workspace.UserGroup) ([]workspace.UserGroup, error) {
		i := slices.IndexFunc(gs, func(g workspace.UserGroup) bool { return g.Name == original })
		if original != "" && i < 0 {
			return nil, fmt.Errorf("personal group %q no longer exists", original)
		}
		var old []string
		if i >= 0 {
			old = gs[i].Problems
		}
		for _, slug := range group.Problems {
			if slices.Contains(old, slug) {
				continue
			}
			if _, err := a.WS.ProblemOf(packName, slug); err != nil {
				return nil, err
			}
		}
		if i >= 0 {
			gs[i] = group
		} else {
			gs = append(gs, group)
		}
		return gs, nil
	})
}
