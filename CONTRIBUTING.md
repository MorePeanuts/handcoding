# Contributing

Thanks for your interest in handcoding.

## Issues first

Open an issue before starting on anything non-trivial. Use the bug report or feature request template. New issues are labeled `needs-discussion` until it is agreed whether and how to address them; this saves work on changes that would not be merged. The plan for each release is in the [milestones](https://github.com/MorePeanuts/handcoding/milestones).

## Development

You need Go (see `go.mod` for the version) and Python 3.8+. The integration tests compile and run real solutions in both languages.

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

`main` is always releasable; releases are tags on `main`. Work happens on short-lived branches that are merged through a pull request and then deleted.

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
