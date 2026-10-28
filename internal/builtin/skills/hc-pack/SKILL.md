---
name: hc-pack
description: Maintain an hc (handcoding) pack as a whole, in a pack checkout (a directory with pack.yaml). Use when landing a finished problem in the pack, placing problems in groups or changing the groups, removing a problem, moving the pack to a new protocol version or bumping the hc version its CI pins, releasing a new version of the pack, when asked to rename a problem's slug or the pack, or when the protocol cannot express a problem. For writing or fixing one problem use hc-problem, and for its judge rule, references and test cases hc-cases.
---

# handcoding pack maintenance

A pack is a set of hc problems published together, usually as a git
repository of its own: `pack.yaml` at the root and one directory per
problem in `problems/<dir>/`. Its checkout is an hc workspace, where the
pack is the only pack and its problems are yours to edit. Users install
the pack into their practice workspaces, and their solutions and history
are keyed by the pack's name and each problem's slug.

This skill covers the pack as a whole. A single problem is written with
the `hc-problem` skill (what it is) and finished with the `hc-cases` skill
(how it is judged); this skill says when it may land and where it goes.

## The spec

The facts are in the protocol that hc embeds. When a step names a section,
read it with `hc docs protocol <section>` first, and follow it exactly;
when this skill and the spec seem to disagree, the spec wins.

- `hc docs protocol packs`: what a pack is and which files belong to it
- `hc docs protocol packyaml`: every field of pack.yaml
- `hc docs protocol groups`: the rules for groups
- `hc docs protocol identity`: slugs, ids and directory names
- `hc docs protocol protocol-versions`: versions and the changelog

## Identity: never rename, never reuse

- **Never rename a problem's slug or its directory, and never change its
  `id`.** Users' solutions and history would no longer find it. When a
  problem has to change that much, add a new problem with a new slug.
- **Never change the pack's `name`** in pack.yaml: installed copies,
  solutions and history are keyed by it. `title` may change.
- **Never reuse the slug of a removed problem for a different problem.**
  Users who solved the old one would see their solutions under the new
  one. Before you pick a slug, check that no problem in the pack's history
  had it: `git log --all --stat --oneline -- 'problems/<slug>/*' 'problems/*-<slug>/*'`
  shows every commit that touched such a directory.

If the user asks for one of these, explain what it breaks and let them
decide; do not do it on your own.

## Workflow: add a problem

1. Check that the slug is free: `hc list --plain` lists the pack's
   problems, and the history must not have it either ([Identity](#identity-never-rename-never-reuse)).
2. Write the problem with `hc-problem` (`hc fetch <slug>` or `hc new <slug>`),
   then finish it with `hc-cases`: judge rule, reference solutions, test
   cases, `hc validate <slug> --run`.
3. Place it in the groups ([Groups](#groups)).
4. Run `hc pack check`. It checks the whole pack as its CI does, and must
   pass. Only ready problems land: never commit a problem that fails it,
   or one without `judge.compare`.
5. Land one problem per commit (or pull request), whose message names the
   problem, with its group changes in the same commit. Commit only the pack's
   files: `pack.yaml` and `problems/<dir>/`, never `solutions/`, `.hc/`,
   `hc.yaml` or `pyrightconfig.json`.

To fix a problem that has landed, use `hc-problem` or `hc-cases` the same
way, then `hc pack check`. Fixing cases, a reference or a statement is not
breaking; changing a signature is, since users' solutions must follow it.

## Groups

Groups only arrange the problems for display (`hc list --group <name>`).
Read `hc docs protocol groups`.

- **A new problem** goes into every group whose topic it fits, possibly
  none. Keep the order the group follows (difficulty, a study plan's
  sequence, ids); append when it follows none. Do not create a group for a
  single problem unless asked.
- **The order of the groups** is the order users see. Change it only when
  asked.
- **Renaming or removing a group** changes only the display: no solution
  or history depends on it, and removing a group keeps its problems. Users
  may filter by a group's `name`, so prefer changing its `title`, and say
  so in the release notes when a `name` changes.
- A group may list only slugs of problems of the pack; when you remove a
  problem, remove its slug from every group.

## Removing a problem

Removing a problem is a breaking change: users lose it from the pack, and
their solutions to it stay behind. Do it only when asked. Delete its
directory, remove its slug from every group, note it for the next release,
and never reuse the slug ([Identity](#identity-never-rename-never-reuse)).
Removing a language from `languages` is breaking too.

## Moving to a new protocol version

Do this when a new hc release implements a newer protocol, or no longer
reads the one the pack declares.

1. Read `hc docs protocol protocol-versions` with the new hc: its table
   says what each version changes.
2. Bump `protocol` in pack.yaml only when a problem is going to use a
   feature the newer version introduced (the Since columns), or when the
   new hc no longer reads the declared version. A higher `protocol` makes
   older hc refuse the pack, so it is a breaking change for users who have
   not upgraded. If the new version changes or removes a feature, update
   every problem that uses it as the changelog says.
3. Bump the hc version the pack's CI installs to the new one, so CI checks
   the pack with the hc that reads its protocol.
4. Run `hc skill install`, so the skills match the new hc, and review the
   diff of the skills (`git diff`, when the pack commits them): from now on
   follow the new versions.
5. Run `hc pack check` with the new hc; it must pass.
6. Land it as one commit of its own, separate from new problems.

## Releasing

`version` in pack.yaml is informational (installs are pinned by commit),
but it is how users and their tools see what changed. Read
`hc docs protocol packyaml` for its format: SemVer without a `v`.

1. Run `hc pack check`; it must pass on the commit you release.
2. Bump `version`: the patch number for fixes, the minor number for new
   problems and groups, the major number for breaking changes (before
   1.0.0, the minor number). Breaking changes are removed problems,
   changed signatures, removed languages and a higher `protocol`.
3. Write the release notes: problems added, fixed and removed, and every
   breaking change with what users should do.
4. Commit, and tag that commit `v<version>`. Push the tag only when the
   user asks.

## When the protocol cannot express a problem

Some problems need what the protocol does not have: an interactive judge,
a type it lacks, a rule no compare mode or checker can state.
`hc docs protocol out-of-scope` lists the known gaps.

**Stop. Never work around it**: no approximated signature, no case that
judges something else, no checker that pretends. Do not add the problem.
Tell the user what is missing and why, and suggest reporting it upstream
as an issue at https://github.com/MorePeanuts/handcoding, with the problem
and what the protocol would need; offer to draft it.

## Summary

Report what changed in the pack: problems added, fixed or removed, group
changes, a protocol or version bump, and every breaking change. Say
whether `hc pack check` passed.
