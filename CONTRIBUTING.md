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

## Pull requests

- Keep each PR to one change, and link the issue it closes.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:` …). The release changelog is grouped by these prefixes.
- If you change the CLI or the problem protocol, update `docs/protocol.md` and the agent skill in `internal/builtin/skill/SKILL.md`. Agents follow the skill literally.
- Until 1.0 there is no backward compatibility to preserve. Prefer the cleanest design over migrations or compatibility shims.
