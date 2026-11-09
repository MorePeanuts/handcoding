package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/skill"
	"github.com/MorePeanuts/handcoding/internal/version"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// keptSkillsPref is the workspace's pref that remembers the edited skill
// copies SyncWorkspace last warned about (see keptKey).
const keptSkillsPref = "kept_skills"

// SkillContext is the context of the workspace's skills: a pack checkout
// gets the authoring set, a practice workspace the practice set.
func (a *App) SkillContext() skill.Context {
	if a.WS.Kind == workspace.Authoring {
		return skill.Authoring
	}
	return skill.Practice
}

// InstallSkills installs skills into the workspace as skill.Install does,
// holding the workspace's skills lock, which it waits for.
func (a *App) InstallSkills(names []string, opts skill.Options) ([]skill.Change, error) {
	unlock, _, err := a.WS.LockSkills(true)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return skill.Install(a.WS.Root, a.SkillContext(), names, opts)
}

// AgentSkills installs the skills an agent that hc gen or hc fix starts
// needs, as hc hcVersion. A practice workspace gets its set as a bare
// `hc skill install` gives it, except that no skill is removed that a user
// installed by name: missing and outdated copies are written, managed
// copies of skills hc no longer ships removed, edited ones kept. A pack
// checkout commits its skills, which its CI requires to be the pinned hc's
// (see InitPack), so it gets only the skills it lacks, and its other files
// stay as they are; PackSkillVersions tells when they are another hc's.
func (a *App) AgentSkills(hcVersion string) ([]skill.Change, error) {
	ctx := a.SkillContext()
	opts := skill.Options{Version: hcVersion, MissingOnly: ctx == skill.Authoring}
	return a.InstallSkills(skill.Set(ctx), opts)
}

// WorkspaceSync is what SyncWorkspace did.
type WorkspaceSync struct {
	// Version is the hc the workspace was brought in step with.
	Version string
	// Changes are the skills and files it changed.
	Changes []skill.Change
	// Kept are the managed skill copies, edited since hc installed them,
	// that miss content hc ships now, when SyncWorkspace has not warned
	// about the same copies before.
	Kept []skill.Installed
}

// SyncWorkspace keeps what hc manages in a practice workspace, its skills
// and its block of AGENTS.md, in step with the running hc, hcVersion. Every
// command that opens a practice workspace calls it, so it writes nothing
// unless something is stale: a managed skill copy is outdated, or
// AGENTS.md's block is not the one hc writes now. Then it syncs as
// AgentSkills does, but creates neither a block of AGENTS.md nor a
// CLAUDE.md that the workspace lacks, which the user may have removed on
// purpose (`hc skill install` creates them).
//
// It only ever upgrades: it does nothing unless hcVersion is a release
// (version.IsRelease), so that a development build and an installed hc do
// not rewrite the workspace back and forth, nor when a managed copy comes
// from a newer hc (version.Newer). It does nothing either while another hc
// process holds the skills lock, which keeps processes that open the
// workspace at once from writing together; one of them syncs.
//
// It returns nil when it did nothing. Edited copies are kept, and reported
// in Kept the first time they are found, remembered in the workspace's
// prefs.
func (a *App) SyncWorkspace(ctx context.Context, hcVersion string) (*WorkspaceSync, error) {
	if a.WS.Kind != workspace.Practice || !version.IsRelease(hcVersion) {
		return nil, nil
	}
	st, err := skill.Check(a.WS.Root, skill.Practice)
	if err != nil || !upgradable(st, hcVersion) {
		return nil, err
	}
	kept := keptCopies(st)
	if !stale(st) && len(kept) == 0 {
		return nil, nil
	}
	unlock, held, err := a.WS.LockSkills(false)
	if err != nil || !held {
		return nil, err
	}
	defer unlock()
	res := &WorkspaceSync{Version: hcVersion}
	// Another process may have synced since the first check.
	if st, err = skill.Check(a.WS.Root, skill.Practice); err != nil || !upgradable(st, hcVersion) {
		return nil, err
	}
	if stale(st) {
		opts := skill.Options{Version: hcVersion, Refresh: true}
		changes, err := skill.Install(a.WS.Root, skill.Practice, skill.Set(skill.Practice), opts)
		for _, c := range changes {
			// What it skipped is either an edited copy, reported in Kept,
			// or a directory hc did not install, the user's.
			if c.Action != skill.Skipped {
				res.Changes = append(res.Changes, c)
			}
		}
		if err != nil {
			return res, err
		}
	}
	res.Kept = a.newlyKept(ctx, keptCopies(st))
	if len(res.Changes) == 0 && len(res.Kept) == 0 {
		return nil, nil
	}
	return res, nil
}

// upgradable reports whether hc hcVersion may write the skills of st: no
// managed copy comes from a newer hc, which would then be downgraded.
func upgradable(st skill.Status, hcVersion string) bool {
	return !slices.ContainsFunc(st.Copies, func(in skill.Installed) bool { return version.Newer(in.Version, hcVersion) })
}

// stale reports whether something hc manages in st is not what it writes
// now.
func stale(st skill.Status) bool {
	return st.IndexStale || slices.ContainsFunc(st.Copies, func(in skill.Installed) bool { return in.State == skill.Outdated })
}

// keptCopies returns the copies in st that the user edited and that miss
// content hc ships now.
func keptCopies(st skill.Status) []skill.Installed {
	var kept []skill.Installed
	for _, in := range st.Copies {
		if in.State == skill.Modified && in.Superseded {
			kept = append(kept, in)
		}
	}
	return kept
}

// newlyKept returns kept unless SyncWorkspace warned about the same copies
// before, and remembers them. A change to any of them (another edit, or
// another version of the skill in hc) is new.
func (a *App) newlyKept(ctx context.Context, kept []skill.Installed) []skill.Installed {
	if len(kept) == 0 {
		return nil
	}
	s, err := a.Store()
	if err != nil {
		// Commands that need the history report it.
		return nil
	}
	key := keptKey(kept)
	if prev, err := s.Pref(ctx, "", keptSkillsPref); err != nil || prev == key {
		return nil
	}
	if err := s.SetPref(ctx, "", keptSkillsPref, key); err != nil {
		return nil
	}
	return kept
}

// keptKey identifies a set of kept copies: each one's directory, the hash of
// its content and that of the skill hc ships.
func keptKey(kept []skill.Installed) string {
	lines := make([]string, len(kept))
	for i, in := range kept {
		shipped := ""
		if s, ok := skill.Get(in.Name); ok {
			shipped = s.Hash
		}
		lines[i] = fmt.Sprintf("%s %s %s", in.Dir, in.Hash, shipped)
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// PackSkillVersions returns, in a pack checkout, the hc versions other than
// hcVersion that installed its managed skills, sorted: when there are any,
// the pack's skills, which its CI requires to be those of the hc it pins,
// are another hc's than the running one, and moving the pack to this hc is
// a step of its own (the hc-pack skill's). It returns nil in a practice
// workspace, and when hcVersion is not a release, which a pack cannot pin.
func (a *App) PackSkillVersions(hcVersion string) []string {
	if a.WS.Kind != workspace.Authoring || !version.IsRelease(hcVersion) {
		return nil
	}
	st, err := skill.Check(a.WS.Root, skill.Authoring)
	if err != nil {
		return nil
	}
	var versions []string
	for _, in := range st.Copies {
		if in.Version != "" && in.Version != hcVersion && !slices.Contains(versions, in.Version) {
			versions = append(versions, in.Version)
		}
	}
	slices.Sort(versions)
	return versions
}
