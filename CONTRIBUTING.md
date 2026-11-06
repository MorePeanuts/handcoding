# Contributing

Thanks for your interest in handcoding.

## Issues first

Open an issue before starting on anything non-trivial. Use the bug report or feature request template. New issues are labeled `needs-discussion` until it is agreed whether and how to address them; this saves work on changes that would not be merged. The plan for each release is in the [milestones](https://github.com/MorePeanuts/handcoding/milestones).

## Development

You need Go (see `go.mod` for the version), git and [uv](https://docs.astral.sh/uv/getting-started/installation/). The integration tests compile and run real solutions in both languages, with the `python3` on PATH, never the virtual environment hc manages for users. Solutions, references and drivers must keep working on the oldest versions hc supports (Python 3.10, Go 1.22; see the protocol's [language versions](docs/protocol.md#language-versions)), so the repository is a minimal uv project (`pyproject.toml`, no dependencies) pinned to Python 3.10 by `.python-version`, and `make test` runs the tests with `uv run`, which puts its `.venv` first on PATH (uv downloads Python 3.10 the first time if you have none). `make test PY=3.13` tests another version, in `.venv-3.13`, and `make test PY=system` the `python3` on PATH as it is. CI runs them on Python 3.10 (Linux) and the latest release (macOS), and `TestGoMinimumVersion` builds the Go side at Go 1.22.

`uv sync` creates the same `.venv`, which pyright (configured in `pyproject.toml`) and editors that detect `.venv` use for hc's Python (`internal/lang/python/*.py`) and the regression set's references, checkers and generators, so they check them against Python 3.10.

`make test` and CI set `HC_TEST_REQUIRE_TOOLS=1`, which makes a test that needs a missing tool, or a `python3` older than 3.10, fail. Plain `go test` skips such tests instead, so it also works without Python; on macOS, whose `python3` is 3.9, that skips almost every integration test. A new test that needs a tool checks it with `internal/testenv`.

```sh
make build      # bin/hc
make install    # go install into $(go env GOBIN)
make test       # unit and end-to-end tests (-race), on Python 3.10
make lint       # go vet + golangci-lint (the version CI uses is in .github/workflows/ci.yml)

uv run go test ./internal/app -run 'TestRegressionReferences/two-sum' -v   # one subtest
```

Use a scratch workspace instead of your own `~/handcoding`:

```sh
go run ./cmd/hc init /tmp/hc-dev
go run ./cmd/hc -w /tmp/hc-dev pack install testdata/regression   # the regression set, copied as an installed pack
go run ./cmd/hc -w /tmp/hc-dev test two-sum
```

The regression set is a pack, so it is also a workspace of its own (a pack checkout, see [working on a pack](docs/how-to-use.md#working-on-a-pack)): after changing its cases, `go run ./cmd/hc -w testdata/regression cases build <slug> --force -l python` regenerates their expected outputs in place. Its `.gitignore` keeps hc's state and your solutions out of git.

The tests of `internal/app` and `internal/tui` run in parallel (`t.Parallel()`), each in its own workspace, and a new one should too, unless it sets the environment (`t.Setenv`, which Go refuses in a parallel test: the LeetCode session and git tests) or times solutions against a time limit (`timelimit_test.go`), which other tests competing for the machine would make flaky. Go runs those first, one at a time, and the parallel ones after them.

## Code layout

```
cmd/hc              entry point
internal/cli        cobra commands
internal/tui        bubbletea interface
internal/app        use cases shared by the CLI and the TUI, including validation and pack management (hc pack)
internal/problem    problem protocol: problem.yaml, cases.jsonl, types
internal/pack       packs: pack.yaml, a pack's problems and their identity
internal/workspace  the workspace (practice, or a pack checkout): its packs, solutions, pack/slug lookup
internal/lang       language interface; golang/ and python/ drivers
internal/git        runs the git executable to install and update packs
internal/runner     build, run the driver, collect results
internal/judge      output comparison
internal/store      submission history (SQLite, pure Go)
internal/leetcode   LeetCode client and conversion
internal/termimg    images in the terminal
internal/agent      coding agent integration
internal/skill      agent skill registry and managed install
internal/builtin    embedded agent skills
internal/regression locates the regression set for tests
internal/testenv    skips (or fails) tests whose tools are missing
testdata/regression the regression set: a pack covering the whole protocol
```

[CLAUDE.md](CLAUDE.md) describes the architecture in more detail.

## Adding a language

A solution language is a driver: it turns a problem's signature into starter code, and into a program that runs the user's code on test cases. The contract between hc and its drivers is internal to hc and is not part of the [problem protocol](docs/protocol.md); it can change in any release. What a solution in a language looks like is part of the protocol, though, so a new language also adds a subsection to the protocol's [Solution ABI](docs/protocol.md#solution-abi).

A language implements the `lang.Language` interface in [`internal/lang/lang.go`](internal/lang/lang.go) and is registered in `app.Languages` (`internal/app`). The interface covers the solution file name, starter code, `Prepare` (writes the driver files next to the solution before every run, idempotently), `Build` (returns `*lang.CompileError` for a Compile Error; it may keep programs in a cache, as Go does in `.hc/cache/build/go/`), the run and debug commands, `Toolchain` (detects the compiler or interpreter and its version) and `Setup` (prepares an environment hc manages, such as Python's `.hc/venv`). Each driver declares `MinVersion`, the oldest toolchain it supports; the code it generates and the code problems ship must work on it.

A driver only **decodes the inputs, calls the user's code and encodes the result**. It never judges: all comparison happens in `internal/judge`, on decoded JSON, so a new language does not reimplement it. Decoding includes building the protocol's structure inputs and `Node` structures; encoding includes `returns_ref`, `judge.output` and `output_length`, and the `returns_copy` check.

- `HC_CASES` names a JSON file holding `[{"id": 1, "input": {...}}, ...]`, the inputs as in `cases.jsonl`.
- `HC_OUT` names a file the driver appends one JSON line to per case:

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  A non-empty `error` is a Runtime Error; keep only the stack frames of the user's code if you can, and start the message with `invalid input:` when the input does not fit its structure. `elapsed_ns` covers only the time spent in the user's code: the function call, or for a class problem the sum of the constructor and method calls, each timed after its arguments are decoded and before its result is encoded. A run's total is the sum of its cases' `elapsed_ns`.
- `HC_TIME_LIMIT_NS`, when set, is the run's [time limit](docs/protocol.md#time-limit) in nanoseconds, which `app` computes from the reference's times (`app.timeLimit`). The driver adds up `elapsed_ns` and stops after the case that brings the total over it. A case may never return, so while the user's code runs a watchdog (a goroutine, a thread) checks the time spent so far: once the total is over the limit, it appends `{"id": 2, "output": null, "elapsed_ns": 5678, "time_limit": true}` for the running case and exits. The clock and the output file are shared with it, under a lock. Judging the time is still the runner's: a case with `time_limit`, or whose `elapsed_ns` brings the total over the limit, is Time Limit Exceeded.
- The user's stdout and stderr go straight to the terminal, so print debugging works.
- Cases run in order. The first case without a result line is Runtime Error, or Time Limit Exceeded when the run timed out, and the cases after it, like the cases after one that exceeded the time limit, are Not Run (`runner.Run`).
- Checkers (`judge.compare: checker`) are not the driver's business either: they are Python for every solution language, run through `lang.Checker`, which the Python language implements, and `judge.Checked` turns their results into verdicts. Nor are generated cases: `runner.Generate` runs the problem's `generator.py` through `lang.Generator` (Python again) before the driver starts, and the driver gets their inputs like any other.

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

### Protocol versions

A [protocol version](docs/protocol.md#protocol-versions) is a draft until the first stable release that implements it, and final from then on. Protocol changes made during a milestone go into its draft version without a bump: protocol 1 is final with `v0.1.0`, and the changes after it go into protocol 2, which `v0.2.0` makes final. A change to a final version, an added feature included, needs a new version. Whether a version is final is `problem.FinalProtocol` (`internal/problem/protocol.go`), and `TestFinalProtocol` (`internal/regression`) fails when the features of a final version change: the types, kinds, compare modes, fields and languages that `TestProtocolCoverage` derives from the code. It cannot see a feature whose meaning changes; review must.

Before tagging a stable release whose protocol is a draft:

1. Set `problem.FinalProtocol` to `problem.Protocol`.
2. Write the version's snapshot: `go test ./internal/regression -run TestFinalProtocol -update`, and commit `internal/regression/testdata/protocol-N.txt`.
3. In the version table of `docs/protocol.md` and `docs/protocol_zh.md`, mark the version final with the release.

The first protocol change after that bumps `problem.Protocol` and adds a row to that table.

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
