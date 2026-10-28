# Contributing

Thanks for your interest in handcoding.

## Issues first

Open an issue before starting on anything non-trivial. Use the bug report or feature request template. New issues are labeled `needs-discussion` until it is agreed whether and how to address them; this saves work on changes that would not be merged. The plan for each release is in the [milestones](https://github.com/MorePeanuts/handcoding/milestones).

## Development

You need Go (see `go.mod` for the version) and Python 3.10+. The integration tests compile and run real solutions in both languages; the Python ones are skipped when `python3` is older than 3.10. They use that `python3` directly, never the virtual environment hc manages for users, so they need no network. Solutions, references and drivers must keep working on the oldest versions hc supports (Python 3.10, Go 1.22; see the protocol's [language versions](docs/protocol.md#language-versions)): CI runs the tests on Python 3.10, and `TestGoMinimumVersion` builds the Go side at Go 1.22.

```sh
make build      # bin/hc
make install    # go install into $(go env GOBIN)
make test       # unit and end-to-end tests (-race)
make lint       # go vet + golangci-lint (the version CI uses is in .github/workflows/ci.yml)
```

Use a scratch workspace instead of your own `~/handcoding`:

```sh
go run ./cmd/hc init /tmp/hc-dev
cp -R testdata/regression /tmp/hc-dev/packs/    # the regression set, as an installed pack
go run ./cmd/hc -w /tmp/hc-dev test two-sum
```

## Code layout

```
cmd/hc              entry point
internal/cli        cobra commands
internal/tui        bubbletea interface
internal/app        use cases shared by the CLI and the TUI
internal/problem    problem protocol: problem.yaml, cases.jsonl, types
internal/pack       packs: pack.yaml, a pack's problems and their identity
internal/workspace  the workspace: hc.yaml, its packs, solutions, pack/slug lookup
internal/lang       language interface; golang/ and python/ drivers
internal/runner     build, run the driver, collect results
internal/judge      output comparison
internal/store      submission history (SQLite, pure Go)
internal/leetcode   LeetCode client and conversion
internal/termimg    images in the terminal
internal/agent      coding agent integration
internal/skill      agent skill registry and managed install
internal/builtin    embedded agent skills
internal/regression locates the regression set for tests
testdata/regression the regression set: a pack covering the whole protocol
```

[CLAUDE.md](CLAUDE.md) describes the architecture in more detail.

## Adding a language

A solution language is a driver: it turns a problem's signature into starter code, and into a program that runs the user's code on test cases. The contract between hc and its drivers is internal to hc and is not part of the [problem protocol](docs/protocol.md); it can change in any release. What a solution in a language looks like is part of the protocol, though, so a new language also adds a subsection to the protocol's [Solution ABI](docs/protocol.md#solution-abi).

A language implements the `lang.Language` interface in [`internal/lang/lang.go`](internal/lang/lang.go) and is registered in `app.Languages` (`internal/app`). The interface covers the solution file name, starter code, `Prepare` (writes the driver files next to the solution before every run, idempotently), `Build` (returns `*lang.CompileError` for a Compile Error), the run and debug commands, `Toolchain` (detects the compiler or interpreter and its version) and `Setup` (prepares an environment hc manages, such as Python's `.hc/venv`). Each driver declares `MinVersion`, the oldest toolchain it supports; the code it generates and the code problems ship must work on it.

A driver only **decodes the inputs, calls the user's code and encodes the result**. It never judges: all comparison happens in `internal/judge`, on decoded JSON, so a new language does not reimplement it. Decoding includes building the protocol's structure inputs and `Node` structures; encoding includes `returns_ref`, `judge.output` and `output_length`, and the `returns_copy` check.

- `HC_CASES` names a JSON file holding `[{"id": 1, "input": {...}}, ...]`, the inputs as in `cases.jsonl`.
- `HC_OUT` names a file the driver appends one JSON line to per case:

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  A non-empty `error` is a Runtime Error; keep only the stack frames of the user's code if you can, and start the message with `invalid input:` when the input does not fit its structure. `elapsed_ns` covers only the time spent in the user's code.
- The user's stdout and stderr go straight to the terminal, so print debugging works.
- Cases without a result line become Runtime Error, or Time Limit Exceeded when the run timed out (`runner.Run`).
- Checkers (`judge.compare: checker`) are not the driver's business either: they are Python for every solution language, run through `lang.Checker`, which the Python language implements, and `judge.Checked` turns their results into verdicts.

The existing drivers are good references: `internal/lang/golang` (generated code plus a static runtime) and `internal/lang/python` (a static runtime with the signature embedded as JSON).

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
- If you change the problem protocol, update its spec, `docs/protocol.md`, and the Chinese translation `docs/protocol_zh.md`. Skills cite the spec by section id (`hc docs protocol <id>`), so keep its headings stable. If you change how hc behaves, update `docs/how-to-use.md` and its Chinese mirror. In both cases update the agent skills in `internal/builtin/skills/`: agents follow them literally, and `TestSkillsLint` fails when a skill mentions a command or flag that no longer exists. `README.md` has a Chinese mirror too.
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
