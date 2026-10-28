package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/git"
	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// The packs of a practice workspace other than local are installed from a
// source, recorded in the packs list of hc.yaml (workspace.PackSource): a
// git URL or repository, cloned, or a plain directory, copied. An installed
// pack is a git checkout of the recorded commit, so updating it is a fetch
// and a checkout.

// PackOptions configure installing and updating packs.
type PackOptions struct {
	// Ref is the tag or branch to install or follow; empty keeps the
	// recorded one, or the default branch.
	Ref string
	// Force discards local changes to a pack being updated.
	Force bool
	// Progress receives git's progress; nil discards it.
	Progress io.Writer
}

// requirePractice refuses pack management in an authoring workspace, whose
// only pack is the checkout itself.
func (a *App) requirePractice() error {
	if a.WS.Kind != workspace.Practice {
		return fmt.Errorf("%s is a pack checkout, which has no installed packs: manage packs in a practice workspace", a.WS.Root)
	}
	return nil
}

// checkPackName refuses names that are not a pack's, such as local or a
// path.
func checkPackName(name string) error {
	if name == pack.LocalName {
		return fmt.Errorf("%s is the workspace's own problems, not an installed pack", pack.LocalName)
	}
	if err := pack.CheckName(name); err != nil {
		return fmt.Errorf("no pack %q: %w", name, err)
	}
	return nil
}

// scpRE matches git's scp-like syntax for SSH URLs, [user@]host:path.
var scpRE = regexp.MustCompile(`^([A-Za-z0-9._-]+@)?[A-Za-z0-9._-]+:[^/\\]`)

// locateSource interprets the source of hc pack install: an existing
// directory, made absolute, which is copied unless it is a git repository;
// otherwise a git URL.
func locateSource(source string) (src string, copied bool, err error) {
	if fi, err := os.Stat(source); err == nil {
		abs, err := filepath.Abs(source)
		if err != nil {
			return "", false, err
		}
		switch {
		case !fi.IsDir():
			return "", false, fmt.Errorf("%s is not a directory: a source is a git URL, a git repository or a pack's directory", abs)
		case git.IsRepository(abs):
			return abs, false, nil
		case exists(filepath.Join(abs, pack.ManifestFile)):
			return abs, true, nil
		}
		return "", false, fmt.Errorf("%s is neither a git repository nor a pack (it has no %s)", abs, pack.ManifestFile)
	}
	if strings.Contains(source, "://") || scpRE.MatchString(source) {
		return source, false, nil
	}
	return "", false, fmt.Errorf("%s: no such directory, and not a git URL", source)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// loadManifest reads and checks the manifest of the pack in dir, from
// source.
func loadManifest(dir, source string) (*pack.Manifest, error) {
	m, err := pack.Load(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s is not a pack: it has no %s", source, pack.ManifestFile)
	}
	if err == nil {
		if err = m.Validate(); err != nil {
			err = fmt.Errorf("%s: %w", pack.ManifestFile, err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	return m, nil
}

// stage fetches the pack of source s into a new temporary directory of
// packs/, which Packs skips since its name starts with a dot: a copy, or a
// clone at s.Commit if it is set, else at the commit that s.Ref names. It
// returns the directory, the pack's manifest and the commit. The caller
// moves or removes the directory.
func (a *App) stage(ctx context.Context, s workspace.PackSource, copied bool, progress io.Writer) (dir string, m *pack.Manifest, commit string, err error) {
	if err := os.MkdirAll(a.WS.PacksPath(), 0o755); err != nil {
		return "", nil, "", err
	}
	tmp, err := os.MkdirTemp(a.WS.PacksPath(), ".tmp-")
	if err != nil {
		return "", nil, "", err
	}
	dir = tmp
	defer func() {
		if err != nil {
			os.RemoveAll(tmp)
		}
	}()
	if copied {
		if err := pack.Copy(s.Source, dir); err != nil {
			return "", nil, "", err
		}
	} else {
		if err := git.Clone(ctx, s.Source, dir, progress); err != nil {
			return "", nil, "", err
		}
		commit = s.Commit
		if commit == "" {
			if commit, err = git.Resolve(ctx, dir, s.Ref); err != nil {
				return "", nil, "", fmt.Errorf("%s: %w", s.Source, err)
			}
		}
		if err := git.Checkout(ctx, dir, commit, false); err != nil {
			return "", nil, "", fmt.Errorf("%s: %w", s.Source, err)
		}
	}
	if m, err = loadManifest(dir, s.Source); err != nil {
		return "", nil, "", err
	}
	return dir, m, commit, nil
}

// InstallResult is a pack that was installed.
type InstallResult struct {
	Manifest *pack.Manifest
	Source   workspace.PackSource
	// Problems is the number of problems that loaded; Failed are those that
	// did not, which do not keep the pack from being installed.
	Problems int
	Failed   []*pack.LoadError
}

// InstallPack installs the pack of source, a git URL, a git repository or a
// pack's directory, at o.Ref (git sources only) or the default branch, as
// packs/<name>/, and records it in hc.yaml. Its manifest must be valid and
// of a protocol this hc reads, and its name must not be taken.
func (a *App) InstallPack(ctx context.Context, source string, o PackOptions) (*InstallResult, error) {
	if err := a.requirePractice(); err != nil {
		return nil, err
	}
	src, copied, err := locateSource(source)
	if err != nil {
		return nil, err
	}
	if copied && o.Ref != "" {
		return nil, fmt.Errorf("%s is a plain directory, which is copied as it is: --ref needs a git source", src)
	}
	s := workspace.PackSource{Source: src, Ref: o.Ref}
	tmp, m, commit, err := a.stage(ctx, s, copied, o.Progress)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	s.Name, s.Commit = m.Name, commit
	dst := a.WS.PackPath(m.Name)
	if exists(dst) {
		rel := filepath.Join(workspace.PacksDir, m.Name)
		if old, ok := a.WS.Config.PackSource(m.Name); ok {
			return nil, fmt.Errorf("the pack %s is already installed in %s, from %s: `hc pack update %s` updates it", m.Name, rel, old.Source, m.Name)
		}
		return nil, fmt.Errorf("%s already exists, and is not in the packs list of %s: `hc pack remove %s` first", rel, workspace.ConfigFile, m.Name)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return nil, err
	}
	list := slices.Clone(a.WS.Config.Packs)
	if i := slices.IndexFunc(list, func(e workspace.PackSource) bool { return e.Name == s.Name }); i >= 0 {
		// Listed but missing: installed anew from this source.
		list[i] = s
	} else {
		list = append(list, s)
	}
	if err := a.WS.SavePacks(list); err != nil {
		os.RemoveAll(dst)
		return nil, err
	}
	return a.installed(dst, m, s)
}

// installed loads the problems of the pack just installed in dir.
func (a *App) installed(dir string, m *pack.Manifest, s workspace.PackSource) (*InstallResult, error) {
	m.Dir = dir
	probs, failed, err := pack.LoadProblems(m.ProblemsPath(), m.Name)
	if err != nil {
		return nil, err
	}
	return &InstallResult{Manifest: m, Source: s, Problems: len(probs), Failed: failed}, nil
}

// InstallMissing installs every pack of hc.yaml's list that is not in
// packs/, at its recorded commit, so that a workspace can be reproduced from
// its hc.yaml. A copied directory is copied again. A pack that fails does
// not keep the others from being installed; the errors are joined.
func (a *App) InstallMissing(ctx context.Context, o PackOptions) ([]*InstallResult, error) {
	if err := a.requirePractice(); err != nil {
		return nil, err
	}
	var (
		results []*InstallResult
		errs    []error
	)
	for _, s := range a.WS.Config.Packs {
		dst := a.WS.PackPath(s.Name)
		if exists(dst) {
			continue
		}
		res, err := a.reinstall(ctx, s, dst, o.Progress)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err))
			continue
		}
		results = append(results, res)
	}
	return results, errors.Join(errs...)
}

func (a *App) reinstall(ctx context.Context, s workspace.PackSource, dst string, progress io.Writer) (*InstallResult, error) {
	tmp, m, _, err := a.stage(ctx, s, s.Copied(), progress)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if m.Name != s.Name {
		return nil, fmt.Errorf("%s now holds the pack %s, not %s", s.Source, m.Name, s.Name)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return nil, err
	}
	return a.installed(dst, m, s)
}

// PackState is how a pack of the workspace came to be there.
type PackState int

const (
	// PackLocal is the workspace's own problems, packs/local.
	PackLocal PackState = iota
	// PackCheckout is the pack an authoring workspace is the checkout of.
	PackCheckout
	// PackManaged is installed and listed in hc.yaml.
	PackManaged
	// PackUnmanaged is a directory of packs/ that hc.yaml does not list,
	// such as a pack copied by hand.
	PackUnmanaged
	// PackMissing is listed in hc.yaml but not installed.
	PackMissing
)

// PackInfo describes a pack of the workspace for hc pack list.
type PackInfo struct {
	Name  string
	State PackState
	// Manifest is nil for a missing pack, or one that cannot be read (Err).
	Manifest *pack.Manifest
	// Source is the pack's entry in hc.yaml (PackManaged and PackMissing).
	Source workspace.PackSource
	// Problems is the number of problems; Solved how many of them have an
	// accepted submission.
	Problems, Solved int
	Err              error
}

// Packs lists the packs of the workspace with their progress: local (or the
// checkout's pack), then the packs of hc.yaml in its order, installed or
// missing, then the directories of packs/ that hc.yaml does not list.
func (a *App) Packs(ctx context.Context) ([]PackInfo, error) {
	packs, err := a.WS.Packs()
	if err != nil {
		return nil, err
	}
	st, err := a.Store()
	if err != nil {
		return nil, err
	}
	statuses, err := st.Statuses(ctx)
	if err != nil {
		return nil, err
	}
	info := func(k *workspace.Pack, state PackState) PackInfo {
		in := PackInfo{Name: k.Name, State: state, Manifest: k.Manifest, Err: k.Err}
		if k.Err != nil {
			return in
		}
		probs, _, err := k.Problems()
		if err != nil {
			in.Err = err
			return in
		}
		in.Problems = len(probs)
		for _, p := range probs {
			if statuses[p.Ref()].Solved {
				in.Solved++
			}
		}
		return in
	}
	byName := map[string]*workspace.Pack{}
	for _, k := range packs {
		byName[k.Name] = k
	}
	var out []PackInfo
	if a.WS.Kind == workspace.Authoring {
		return []PackInfo{info(packs[0], PackCheckout)}, nil
	}
	out = append(out, info(byName[pack.LocalName], PackLocal))
	listed := map[string]bool{pack.LocalName: true}
	for _, s := range a.WS.Config.Packs {
		listed[s.Name] = true
		k, ok := byName[s.Name]
		if !ok {
			out = append(out, PackInfo{Name: s.Name, State: PackMissing, Source: s})
			continue
		}
		in := info(k, PackManaged)
		in.Source = s
		out = append(out, in)
	}
	for _, k := range packs {
		if !listed[k.Name] {
			out = append(out, info(k, PackUnmanaged))
		}
	}
	return out, nil
}

// ManagedPacks are the names of the packs in the packs list of hc.yaml, in
// its order.
func (a *App) ManagedPacks() ([]string, error) {
	if err := a.requirePractice(); err != nil {
		return nil, err
	}
	names := make([]string, len(a.WS.Config.Packs))
	for i, s := range a.WS.Config.Packs {
		names[i] = s.Name
	}
	return names, nil
}

// PackUpdate is what updating a pack changed.
type PackUpdate struct {
	Name string
	// From and To are the pack's hc.yaml entry before and after.
	From, To workspace.PackSource
	// Added and Removed are slugs of problems.
	Added, Removed []string
	// Orphans are solutions and history that no problem of the pack uses
	// any more: of removed problems, or of problems whose directory was
	// renamed. hc keeps them.
	Orphans []Orphan
	// Failed are the problems that fail to load after the update.
	Failed []*pack.LoadError
}

// Orphan is what the user has of a problem that an update removed or moved.
type Orphan struct {
	Slug string
	// Solutions are the solution directories, relative to the workspace
	// root.
	Solutions []string
	// Submissions is the number of submissions in the history (for a
	// removed problem).
	Submissions int
}

// UpdatePack updates the installed pack called name from its source: a git
// pack is fetched and checked out at the latest commit of its ref (o.Ref
// replaces it), and a copied one is copied again. A checkout with local
// changes is refused unless o.Force, which discards them. An update whose
// pack.yaml cannot be read, or that renames the pack, is undone.
func (a *App) UpdatePack(ctx context.Context, name string, o PackOptions) (*PackUpdate, error) {
	if err := a.requirePractice(); err != nil {
		return nil, err
	}
	if err := checkPackName(name); err != nil {
		return nil, err
	}
	dir := a.WS.PackPath(name)
	rel := filepath.Join(workspace.PacksDir, name)
	s, ok := a.WS.Config.PackSource(name)
	switch {
	case !ok && exists(dir):
		return nil, fmt.Errorf("%s is not in the packs list of %s, so hc does not know where it came from: `hc pack remove %s`, then install it with `hc pack install <source>`", rel, workspace.ConfigFile, name)
	case !ok:
		return nil, fmt.Errorf("no pack %s in the packs list of %s", name, workspace.ConfigFile)
	case !exists(dir):
		return nil, fmt.Errorf("%s is missing: `hc pack install` installs it", rel)
	}
	before, _, err := pack.LoadProblems(filepath.Join(dir, pack.ProblemsDir), name)
	if err != nil {
		return nil, err
	}
	u := &PackUpdate{Name: name, From: s, To: s}
	if s.Copied() {
		if o.Ref != "" {
			return nil, fmt.Errorf("%s was copied from the directory %s: --ref needs a git source", name, s.Source)
		}
		if err := a.recopy(ctx, s, dir); err != nil {
			return nil, err
		}
	} else {
		if u.To, err = a.pull(ctx, s, dir, o); err != nil {
			return nil, err
		}
	}
	after, failed, err := pack.LoadProblems(filepath.Join(dir, pack.ProblemsDir), name)
	if err != nil {
		return nil, err
	}
	u.Failed = failed
	if err := a.diff(ctx, u, before, after); err != nil {
		return nil, err
	}
	if u.To != u.From {
		list := slices.Clone(a.WS.Config.Packs)
		for i := range list {
			if list[i].Name == name {
				list[i] = u.To
			}
		}
		if err := a.WS.SavePacks(list); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// recopy replaces the copied pack in dir with a new copy of its source.
func (a *App) recopy(ctx context.Context, s workspace.PackSource, dir string) error {
	tmp, m, _, err := a.stage(ctx, s, true, nil)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if m.Name != s.Name {
		return fmt.Errorf("%s now holds the pack %s, not %s: renaming a pack is a breaking change, so remove %s and install it again", s.Source, m.Name, s.Name, s.Name)
	}
	old := tmp + "-old"
	if err := os.Rename(dir, old); err != nil {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		_ = os.Rename(old, dir)
		return err
	}
	return os.RemoveAll(old)
}

// pull fetches the git pack in dir and checks out the latest commit of its
// ref, and returns its new entry.
func (a *App) pull(ctx context.Context, s workspace.PackSource, dir string, o PackOptions) (workspace.PackSource, error) {
	rel := filepath.Join(workspace.PacksDir, s.Name)
	if !git.IsRepository(dir) {
		return s, fmt.Errorf("%s is not a git checkout: `hc pack remove %s`, then install it again", rel, s.Name)
	}
	changes, err := git.Changes(ctx, dir)
	if err != nil {
		return s, err
	}
	if len(changes) > 0 && !o.Force {
		return s, fmt.Errorf("%s has local changes (%d file(s)), but installed packs are read-only: "+
			"`hc pack update --force %s` discards them\n  %s", rel, len(changes), s.Name, strings.Join(changes, "\n  "))
	}
	if err := git.Fetch(ctx, dir, s.Source, o.Progress); err != nil {
		return s, err
	}
	if o.Ref != "" {
		s.Ref = o.Ref
	}
	commit, err := git.Resolve(ctx, dir, s.Ref)
	if err != nil {
		return s, fmt.Errorf("%s: %w", s.Source, err)
	}
	old, err := git.Head(ctx, dir)
	if err != nil {
		return s, err
	}
	if err := git.Checkout(ctx, dir, commit, o.Force); err != nil {
		return s, err
	}
	m, err := loadManifest(dir, s.Source)
	if err == nil && m.Name != s.Name {
		err = fmt.Errorf("the pack is now called %s: renaming a pack is a breaking change, so remove %s and install it again", m.Name, s.Name)
	}
	if err != nil {
		if uerr := git.Checkout(ctx, dir, old, true); uerr != nil {
			return s, fmt.Errorf("%w (and going back to %s failed: %w)", err, short(old), uerr)
		}
		return s, fmt.Errorf("%s stays at %s: %w", rel, short(old), err)
	}
	s.Commit = commit
	return s, nil
}

// short abbreviates a commit hash.
func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// diff records in u the problems added and removed between before and
// after, and the user's solutions and history that the update orphaned.
func (a *App) diff(ctx context.Context, u *PackUpdate, before, after []*problem.Problem) error {
	afterBySlug := map[string]*problem.Problem{}
	for _, p := range after {
		afterBySlug[p.Slug] = p
	}
	beforeSlugs := map[string]bool{}
	for _, p := range before {
		beforeSlugs[p.Slug] = true
	}
	for _, p := range after {
		if !beforeSlugs[p.Slug] {
			u.Added = append(u.Added, p.Slug)
		}
	}
	var statuses map[string]int
	for _, p := range before {
		q, kept := afterBySlug[p.Slug]
		if kept && filepath.Base(q.Dir) == filepath.Base(p.Dir) {
			continue
		}
		if !kept {
			u.Removed = append(u.Removed, p.Slug)
		}
		o := Orphan{Slug: p.Slug}
		for _, l := range a.Langs.All() {
			if sol := a.WS.SolutionDir(p, l.ID()); exists(sol) {
				o.Solutions = append(o.Solutions, a.WS.Rel(sol))
			}
		}
		if !kept {
			if statuses == nil {
				st, err := a.Store()
				if err != nil {
					return err
				}
				all, err := st.Statuses(ctx)
				if err != nil {
					return err
				}
				statuses = map[string]int{}
				for ref, s := range all {
					statuses[ref] = s.Attempts
				}
			}
			o.Submissions = statuses[p.Ref()]
		}
		if len(o.Solutions) > 0 || o.Submissions > 0 {
			u.Orphans = append(u.Orphans, o)
		}
	}
	return nil
}

// RemoveResult is what removing a pack deleted.
type RemoveResult struct {
	// Deleted reports whether packs/<name> was deleted, and Unlisted
	// whether its entry left hc.yaml.
	Deleted, Unlisted bool
	// Solutions reports whether solutions/<name> was deleted, and
	// Submissions how many submissions were (with purge).
	Solutions   bool
	Submissions int64
}

// RemovePack deletes the installed pack called name, packs/<name>/, and its
// entry in hc.yaml. The user's solutions and history of its problems stay,
// unless purge, which deletes solutions/<name>/ and the pack's submissions.
func (a *App) RemovePack(ctx context.Context, name string, purge bool) (*RemoveResult, error) {
	if err := a.requirePractice(); err != nil {
		return nil, err
	}
	if name == pack.LocalName {
		return nil, fmt.Errorf("%s is the workspace's own problems and cannot be removed", pack.LocalName)
	}
	if err := checkPackName(name); err != nil {
		return nil, err
	}
	dir := a.WS.PackPath(name)
	_, listed := a.WS.Config.PackSource(name)
	if !listed && !exists(dir) {
		return nil, fmt.Errorf("no pack %s in this workspace", name)
	}
	res := &RemoveResult{}
	if exists(dir) {
		if err := os.RemoveAll(dir); err != nil {
			return res, err
		}
		res.Deleted = true
	}
	if listed {
		list := slices.DeleteFunc(slices.Clone(a.WS.Config.Packs), func(s workspace.PackSource) bool { return s.Name == name })
		if err := a.WS.SavePacks(list); err != nil {
			return res, err
		}
		res.Unlisted = true
	}
	if !purge {
		return res, nil
	}
	sol := filepath.Join(a.WS.Root, workspace.SolutionsDir, name)
	if exists(sol) {
		if err := os.RemoveAll(sol); err != nil {
			return res, err
		}
		res.Solutions = true
	}
	st, err := a.Store()
	if err != nil {
		return res, err
	}
	res.Submissions, err = st.DeletePack(ctx, name)
	return res, err
}

// CheckPack checks the pack of an authoring workspace as its CI would
// before publishing it: hc validate --run of every problem (see
// ValidateWorkspace), where a problem that is not ready, or a reference that
// cannot be run, is an error too.
func (a *App) CheckPack(ctx context.Context, o ValidateOptions, report func(Finding)) (ValidateSummary, error) {
	if a.WS.Kind != workspace.Authoring {
		return ValidateSummary{}, fmt.Errorf("%s is a practice workspace: hc pack check checks a pack checkout, so run it in one, or pass its directory", a.WS.Root)
	}
	o.Run, o.Strict = true, true
	return a.ValidateWorkspace(ctx, o, report)
}
