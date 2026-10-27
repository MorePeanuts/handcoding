# Contributing

Thanks for your interest in handcoding.

## Issues first

Open an issue before starting on anything non-trivial. Use the bug report or feature request template. New issues are labeled `needs-discussion` until it is agreed whether and how to address them; this saves work on changes that would not be merged. The plan for each release is in the [milestones](https://github.com/MorePeanuts/handcoding/milestones).

## Development

You need Go (see `go.mod` for the version) and Python 3.10+. The integration tests compile and run real solutions in both languages; the Python ones are skipped when `python3` is older than 3.10. Solutions, references and drivers must keep working on the oldest versions hc supports (Python 3.10, Go 1.22; see the protocol's language versions): CI runs the tests on Python 3.10, and `TestGoMinimumVersion` builds the Go side at Go 1.22.

```sh
make build      # bin/hc
make install    # go install into $(go env GOBIN)
make test       # unit and end-to-end tests (-race)
make lint       # go vet + golangci-lint (the version CI uses is in .github/workflows/ci.yml)
```

Use a scratch workspace instead of your own `~/handcoding`:

```sh
go run ./cmd/hc init /tmp/hc-dev
go run ./cmd/hc -w /tmp/hc-dev test two-sum
```

## Code layout

```
cmd/hc              entry point
internal/cli        cobra commands
internal/tui        bubbletea interface
internal/app        use cases shared by the CLI and the TUI
internal/problem    problem protocol: problem.yaml, cases.jsonl, types
internal/lang       language interface; golang/ and python/ drivers
internal/runner     build, run the driver, collect results
internal/judge      output comparison
internal/store      submission history (SQLite, pure Go)
internal/leetcode   LeetCode client and conversion
internal/termimg    images in the terminal
internal/agent      coding agent integration
internal/builtin    embedded sample problems and agent skill
```

[CLAUDE.md](CLAUDE.md) describes the architecture in more detail.

## Branches

`main` is always releasable; releases are tags on `main`, except patch releases from a maintenance branch (see [Versions and releases](#versions-and-releases)). Work happens on short-lived branches that are merged through a pull request and then deleted.

| Change | Where |
|---|---|
| Features, bug fixes, refactors, dependency updates | Branch + PR |
| CI and release configuration | Branch + PR, so the workflows run before merging |
| Substantial documentation (e.g. README rework) | Branch + PR, to review the rendered result |
| Typos, wording, broken links | Directly on `main` |
| Experiments | `exp/` branch, optionally a draft PR. Delete it when abandoned and note the outcome in the issue |

Name branches `<type>/<issue>-<summary>`, with the same types as commit messages: `feat/18-wezterm-images`, `fix/23-clip-path`, `docs/readme-badges`, `exp/iterm2-preview`.

## Pull requests

- Keep each PR to one logical change, and link the issue it closes (`Closes #N`).
- PR titles follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:` …). The release changelog is grouped by these prefixes.
- PRs are **squash merged** by default: the PR title becomes the commit on `main`, and the individual commits stay visible on the PR. If a change contains parts that should be separate commits (say, a refactor and a feature built on it), prefer splitting it into separate PRs. Rebase merge is the exception, for a series of commits that must land together and are each clean and conventionally titled. Merge commits are disabled.
- If you change the CLI or the problem protocol, update `docs/protocol.md` (and its Chinese mirror `docs/protocol_zh.md`) and the agent skill in `internal/builtin/skill/SKILL.md`. Agents follow the skill literally. `README.md` and `docs/how-to-use.md` have Chinese mirrors too.
- Until 1.0 there is no backward compatibility to preserve. Prefer the cleanest design over migrations or compatibility shims.

## Versions and releases

Versions are [SemVer](https://semver.org/) tags with a `v` prefix. Pushing a tag runs the release workflow.

| Change | Version |
|---|---|
| Fixes and small changes to the released line | Patch: `v0.1.1`, `v0.1.2` … |
| Features for the next milestone, while it is in progress | Pre-release: `v0.2.0-alpha.1`, `v0.2.0-alpha.2` … |
| The milestone is done | `v0.2.0` |

Patch releases contain only fixes, never new features or problem protocol changes. Features accumulate on `main` and ship as pre-releases when there is something worth trying. There are no `-rc` releases.

### Maintenance branches

As long as `main` holds nothing unreleased beyond fixes, a patch release is tagged on `main` and there is no maintenance branch. Once `main` has features for the next milestone and a fix has to ship before them, create a maintenance branch named after the line, from its release tag:

```sh
git switch -c v0.1.x v0.1.0
git push -u origin v0.1.x
```

- A fix lands on `main` first, through the usual PR. Then cherry-pick it (`git cherry-pick -x`) onto a branch from `v0.1.x` and open a PR against `v0.1.x`. CI runs there too: the same change on older code can break in ways `main`'s CI never saw.
- A fix for code that no longer exists on `main` goes to `v0.1.x` directly, through a PR.
- Tag the patch release (`v0.1.1`) on `v0.1.x` once its CI is green.
- Never merge a maintenance branch into `main`, and never delete it.
- Only the latest line is maintained. Once `v0.2.0` ships, `v0.1.x` gets no more releases.

CI runs on every pull request and on every push to `main` and to `v*.x` branches.
