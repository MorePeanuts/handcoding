# Using handcoding

English · [简体中文](how-to-use_zh.md)

- [Workspace](#workspace)
- [The interactive list](#the-interactive-list)
- [Commands](#commands)
- [Getting problems](#getting-problems)
- [Packs](#packs)
- [Test cases from a coding agent](#test-cases-from-a-coding-agent)
- [Debugging](#debugging)
- [Configuration](#configuration)
- [Current limitations](#current-limitations)

## Workspace

`hc init` creates an empty workspace, by default `~/handcoding`, and suggests installing the official LeetCode pack, [leetcode.hc](https://github.com/MorePeanuts/leetcode.hc) (see [Packs](#packs)):

```
~/handcoding/
  hc.yaml                                  configuration, and the list of installed packs
  packs/local/problems/0001-two-sum/       your own problems (see the problem protocol)
  packs/<name>/                            installed packs (read-only)
  solutions/local/go/0001-two-sum/         your code: solution.go, plus generated go.mod and zz_hc_driver.go
  solutions/local/python/0001-two-sum/     your code: solution.py, plus a generated _hc_runner.py
  .hc/hc.db                                submission history (SQLite)
  .hc/venv/                                Python virtual environment managed by hc
  .hc/runtime/python/                      hc_runtime.py, shared by every Python solution
  .hc/cache/                               Go and uv caches for coding agents, generated cases (cases/), Go programs (build/), reference times (baseline/)
  pyrightconfig.json  ty.toml              point pyright and ty at .hc/venv
  AGENTS.md  CLAUDE.md                     a guide for coding agents helping you practice (see Agent skills)
  .claude/  .agents/                       the agent skills
```

`hc init --lang python` makes Python the default solution language (`language` in `hc.yaml`; default `go`). Whatever the language, `hc init` creates the Python environment described below, since most problems need Python even when solved in Go. If it cannot, because neither uv nor Python 3.12+ is installed, `hc init` still creates the workspace, then prints how to get one and exits with a non-zero status; install it and run `hc env sync` in the workspace.

`hc` finds the workspace from `-w <dir>`, then `$HC_WORKSPACE`, then the nearest parent directory containing `hc.yaml` or `pack.yaml` (a pack checkout, see [working on a pack](#working-on-a-pack)), then `~/handcoding`. You can run it from anywhere.

### Packs in a workspace

The problems of a workspace come in [packs](protocol.md#packs), each a directory of `packs/`:

- `packs/local/` holds your own problems: what `hc fetch`, `hc new` and `hc import` add. It has no `pack.yaml` (`local` is a reserved name), and is solved in every language. Its statements are in the [locales](#statements-in-several-languages) that `locales` in `hc.yaml` sets, English (`[en]`) by default.
- Every other directory is an installed pack: a `pack.yaml` whose `name` is the directory's name, and its `problems/`. Installed packs are **read-only**: `hc gen`, `hc fix` and `hc cases add|rm|edit|build` refuse to change them, since updating the pack would replace the change. A pack whose `pack.yaml` lists `languages` can only be solved in those: `hc start -l` refuses the others, and if your default language is not among them, hc uses the pack's first language and says so. `hc pack install` installs them (see [Packs](#packs)); a pack copied into `packs/` by hand works too, and `hc pack list` shows it as unmanaged.

A pack that hc cannot read (no `pack.yaml`, an invalid one, or one written for a newer protocol, which says to upgrade hc) is reported by `hc validate` and the list, and its problems are left out; the rest of the workspace works.

Solutions live in `solutions/<pack>/<lang>/<dir>/`, and the history keeps each submission's pack and slug, so the same slug in two packs is two problems.

A workspace made by an older hc keeps its problems in `problems/` instead of `packs/`. hc stops there and says so: create a new workspace with `hc init <dir>` and move your problems into its `packs/local/problems/`. Solutions and history are not carried over.

Each Go problem is a standalone module, and the driver sits next to `solution.go`. So gopls, `go build` and Delve breakpoints all work on your real file. The generated files are rewritten before every run, so don't edit them. `go.mod` follows your Go: its `go` line is your toolchain's version (`go 1.27` for Go 1.27.2), so your solutions can use everything it supports. hc builds with `GOTOOLCHAIN=local` and never downloads another toolchain. It keeps the programs it builds in `.hc/cache/build/go/`, by the content of the files they are built from, so a run of unchanged code builds nothing; on macOS that also saves the check the system makes of every new program on its first launch. The newest 32 are kept.

Python runs in a virtual environment that hc manages in `.hc/venv`: Python solutions, and the [generators](protocol.md#generators) and [checkers](protocol.md#checkers) that most problems have, whatever the solution's language. `hc init` and `hc pack init` create it, and so does the first command that needs Python when it is missing (a cloned pack checkout, say: `hc start`, `test`, `submit`, `debug`, `cases build`, `validate --run`, `gen`, the TUI's runs):

1. with [uv](https://docs.astral.sh/uv/) if it is on PATH: `uv venv --python '>=3.12'`, which downloads a Python if none is installed, then debugpy with `uv pip`;
2. otherwise with the newest Python 3.12+ on PATH (`python3`, `python` or `python3.N`) and `-m venv`. Debian and Ubuntu (WSL included) ship the part of `venv` that installs pip apart, as the `python3.X-venv` package; without it, hc creates the environment with `-m venv --without-pip`. Judging needs no third-party package, so that environment works; only debugpy, for headless debugging, is missing, and hc warns, naming the package to install (or install uv), then run `hc env sync`;
3. otherwise hc stops with a guide: Python 3.12+ is needed, and why; the commands that install uv on your system (`brew install uv` or the install script on macOS, the install script on Linux, `winget install --id=astral-sh.uv -e` or the PowerShell installer on Windows) and the [uv installation page](https://docs.astral.sh/uv/getting-started/installation/); or install Python 3.12+; or set `python.interpreter`; then run `hc env sync`. It says what it found (`/usr/bin/python3 is 3.9.6`). hc prints the commands; it never runs an installer.

Every command that needs Python reports this the same way, saying first what needs it (`judging two-sum with checker.py needs Python 3.12 or later: …`), and the TUI shows it in the run's output. `hc validate` and `hc pack check` report it in every finding it affects by its first line alone, and print the guide once, after the findings.

In a pack checkout (see [authoring a pack](packs.md)), the environment has Python 3.12 instead, the oldest the protocol supports, which the pack's references, checkers and generators must run on: `uv venv --python 3.12`, or without uv the newest `python3.12` on PATH. So `hc validate --run` and `hc pack check` run them on the same Python as the pack's CI. An environment of another version is created again; when there is no Python 3.12 to be had (no uv, none on PATH), hc uses the newest and warns.

The environment holds debugpy for headless debugging. Downloading Python and installing debugpy need network access, once; judging stays offline. If the environment stops working (the workspace was moved, or its Python removed), hc creates it again. So it does when its Python is older than 3.12 (made by an earlier hc, whose minimum was lower) and uv or a Python 3.12+ on PATH can make a new one; otherwise hc keeps it, and `hc test`, `submit` and `debug` warn that it is too old. `hc env sync` recreates it on demand: after installing uv or Python when hc could not create it, or to get debugpy. Python solutions import `hc_runtime` (`ListNode`, `TreeNode` and the other types), which hc keeps once per workspace in `.hc/runtime/python/`, not next to each solution. The environment puts that directory on its `sys.path` (`hc_runtime.pth`), and hc writes `pyrightconfig.json` and `ty.toml` at the workspace root, unless you have them, so that pyright, basedpyright and ty check your solutions with the same interpreter and resolve `hc_runtime`. Another editor or language server finds it once you point it at `.hc/venv`'s Python. To use your own Python instead (conda, a custom build), set `python.interpreter` in `hc.yaml`; hc then uses it as it is, creates no environment and shows no guide, and writes no editor configuration: to have your editor resolve `hc_runtime`, add `.hc/runtime/python` to its search paths (`extraPaths` for pyright, `environment.extra-paths` for ty).

The oldest supported toolchains are Python 3.12 and Go 1.22 (the protocol's [language versions](protocol.md#language-versions)). Your own solutions may use anything your toolchain supports: `hc test`, `hc submit` and `hc debug` name the toolchain they run with (`[Python 3.12.9]`) and only warn when it is older. Code that problems ship must work on the oldest versions, so `hc cases build` and `hc validate --run` refuse to run reference solutions on an older toolchain. A newer one is fine: `hc validate --run` (and so `hc pack check`) also checks every Go reference against Go 1.22, by vetting it in a module whose go directive is `go 1.22`, which reports newer language features and standard library symbols.

Consider running `git init` in the workspace to keep a record of your practice. The `.gitignore` that `hc init` writes leaves out `.hc/`, the installed packs, which `hc pack install` reinstalls from `hc.yaml`, and the files hc writes next to your solutions before every run (`zz_hc_driver.go`, `go.mod`, `_hc_runner.py`, `__pycache__/`, and the `hc_runtime.py` that versions before v0.1.0-alpha.12 wrote there), so the repository holds only your own code. These rules are a block between `# hc:begin` and `# hc:end`, which a newer hc refreshes when it opens the workspace; add your own rules outside it. A `.gitignore` without the block, such as one written by an hc before v0.1.0-alpha.12, is yours and hc leaves it alone: to get the block, copy it from a new workspace (`hc init` in a temporary directory), and remove the generated files already committed with `git rm -r --cached solutions` followed by `git add solutions`.

### Working on a pack

To start a pack of your own, run `hc pack init`: it writes `pack.yaml`, a `.gitignore`, a CI workflow and the agent files, ready to commit. [Writing packs](packs.md) is the guide to creating, checking and publishing one.

A pack checkout, that is a directory with `pack.yaml` such as a clone of a pack's repository, is a workspace as it is: run hc in it or in any directory below it, or point `-w` at it. It needs no `hc init`, and `hc init` refuses a directory inside a pack. In this authoring workspace the pack is the only pack, and it is writable: `hc fetch`, `hc new` and `hc import` add problems to its `problems/`, and `hc gen`, `hc fix` and `hc cases` change them. So the authoring flow needs no flags: `hc fetch two-sum`, `hc gen two-sum`, `hc validate --run`, and `hc pack check` before publishing (see [checking a pack](#checking-a-pack)). There is no `local` pack, a `packs/` directory in the checkout is ignored, and problems are named by their slug alone.

```
my-pack/
  pack.yaml  problems/                     the pack (see the problem protocol)
  hc.yaml                                  optional: your settings (language, editor, agent, timeout, python)
  solutions/<pack>/<lang>/<dir>/           your solutions, as in a practice workspace
  .hc/  pyrightconfig.json  ty.toml        hc's state, as in a practice workspace
  .claude/  .agents/  AGENTS.md  CLAUDE.md   the agent skills and instructions, once a command installs them
```

Without `hc.yaml` hc uses the defaults, and it never creates one in a checkout. Its `locales` are those of `pack.yaml`, so an `hc.yaml` there must not set them, as it must not list `packs`. A `pack.yaml` that hc cannot read, for example one of a newer protocol, does not keep the workspace from opening: `hc validate` says what is wrong. What hc adds is yours, not the pack's, so keep it out of the repository's history:

```
# .gitignore
.hc/
solutions/
hc.yaml
pyrightconfig.json
ty.toml
```

`hc pack init` writes this `.gitignore`, and commits to keeping `.claude/`, `.agents/` and `AGENTS.md` in the repository, so that a fresh clone describes itself to agents. In a pack created otherwise, ignore them too, or commit them (hc changes only its own skills and its block of `AGENTS.md`).

A pack installed in a practice workspace (`packs/<name>/`, beside `hc.yaml`) is not a checkout: hc run inside it uses the enclosing workspace, where the pack stays read-only.

## The interactive list

Run `hc` (or `hc list`) to open the problem list. It works best with the terminal split in two: `hc` on one side, your editor on the other.

- Press `enter` on a problem to open it. The statement is shown, the solution file is created from the template, and its absolute path is **copied to the clipboard**. In the editor, `:e <paste>` and start writing. To read a statement without starting the problem, press `v` instead: nothing is created or copied until an action such as `e`, `t` or `y` needs the solution.
- Test, submit and debug are available from the statement and the result screen, not from the list, as on LeetCode. Results appear in the TUI, with your program's own output listed separately below them.
- `h` on the list shows every submission of the workspace, newest first, with its problem; `o` there opens the submission's problem, and `esc` comes back. `h` in a problem (the statement or the result screen) shows only that problem's submissions.
- The debugger takes over the terminal. The screen is cleared before it starts, and the TUI comes back when it exits. `d` debugs the first case your code fails, as [`hc debug`](#debugging) picks it. In the history, `d` restores the submission and debugs its first failure.
- Statement images are drawn in the terminal. kitty and Ghostty show the original image. Other truecolor terminals show a block-character approximation that loses detail; press `o` to open the original in your image viewer. See [`images`](#configuration) to choose the mode.
- Statement colors follow the terminal background. When the terminal switches between light and dark while the TUI runs, the statement follows it in terminals that report the switch (Ghostty, kitty 0.38 and later, among others); elsewhere its text still follows, its headings and code keep their colors until you reopen hc. If they look wrong, set `theme` in `hc.yaml`.
- The list shows one pack at a time. With more than one pack, a tab bar at the top names each pack with its progress (solved/total), and `tab` / `shift+tab` switch packs; on a narrow terminal the bar scrolls to keep the current pack in view. A pack without problems, such as `local` before you add one, gets no tab, unless no pack has problems yet.
- `m` stars a problem you want to come back to, and `m` again unstars it. Starred problems show `★` in the list and the statement's title, and the `f` filter can list only them. Stars are kept in `.hc/hc.db`, and survive discarding a problem's solution.
- `c` chooses a group of the pack, or all of its problems, and shows the progress of each. A group lists its problems in the group's order. Search and the filter apply to the pack and group shown.
- With more than one pack that has problems, `hc` opens on a pack picker that lists them with their progress, the pack you were in last under the cursor. `enter` opens a pack's list, and `esc` on the list comes back to the picker. With one such pack, `hc` opens its list directly.
- `hc --pack <name>` (or `hc list --pack <name>`) skips the picker and opens that pack, even an empty one, and `--group <name>` that group (a group name that only one pack uses needs no `--pack`). Each pack opens on the group you chose last.
- `i` switches the statement language of a pack whose statements come in [several languages](#statements-in-several-languages): titles in the list, and the statement on the detail screen. The list header and the statement's title line show the one in use.

| Key | Action | Key | Action |
|---|---|---|---|
| `enter` | Open the problem (and copy the solution path) | `e` | Open the solution in your editor |
| `t` | Test on the examples (in a problem) | `s` | Submit, all cases (in a problem) |
| `d` | Debug the first failing case (in a problem) | `h` | Submission history (the workspace's on the list, the problem's in a problem): code, judge details, restore, debug, delete |
| `y` | Copy the solution path again | `L` | Switch language |
| `r` | Reset to the template | `D` | Discard: delete the solution and its history, back to not started |
| `/` | Search | `f` | Filter: all / unsolved / solved / starred |
| `tab` / `shift+tab` | Next / previous pack | `c` | Choose a group of the pack |
| `v` | View the statement only | `o` | Open the statement's images (statement view); the submission's problem (workspace history) |
| `i` | Switch the statement language | `esc` / `q` | Back (`esc` on the list: to the pack picker); `q` on the list quits |
| `m` | Star or unstar the problem (`★` in the list) | `ctrl+c` | Quit |
| `?` | Every key of the screen (the bottom line shows the common ones) | | |

## Commands

```
Practice       list  start  edit  show  test  submit  debug  reset  discard  history  leetcode
Problems       init  fetch  new  import  cases  validate  pack  docs
Coding agent   gen  fix  skill
```

Problems can be referred to by slug, slug prefix, number or directory name: `hc test 1`, `hc test two`. These search every pack. `<pack>/<ref>` searches one: `hc test local/two-sum`, `hc test leetcode/1`. When a slug (or number) is in several packs, a bare reference is an error that lists them as `pack/slug`, and a prefix must match one slug across all packs.

```sh
hc start two-sum -e                                 # create the solution and open the editor
hc test two-sum                                     # run the examples (an error if no case is marked as one)
hc test two-sum -c 4 -c 5                           # run chosen cases
hc test two-sum --all -v                            # all cases, full output, not recorded
hc test two-sum -i '{"nums":[1,2,3],"target":5}'    # custom input (expected output from the reference, if any)
hc test lru-cache -l py                             # solve in Python
hc test two-sum --all --file draft.py               # run another solution file instead of yours
hc submit two-sum                                   # all cases, recorded in history

hc list --plain --pack leetcode --group arrays   # a pack's group, in its order, as a table

hc history                  # every submission (problems as pack/slug when there are several packs)
hc history two-sum          # submissions of a problem
hc history show 12          # code and judge report of a submission
hc history restore 12       # make a past submission your current code
hc history rm 12 13         # delete submissions
hc reset two-sum            # back to the starter template
hc discard two-sum          # delete the solution and all its submissions
```

Run `hc <command> --help` for every flag.

The report shows how long each case took in your code: the call of the function, or for a design problem the constructor and every method call, without decoding their arguments and encoding their results. The total at the bottom is the sum of these times, so it leaves out starting the program and reading the cases.

`--file` runs a solution file kept anywhere instead of the one in `solutions/`, so you can compare drafts or check that a slow solution exceeds the [time limit](#time-limit) (`--file` with the reference's own file shows its times). Its language comes from its extension, unless `-l` says otherwise. The file is run alone, copied into a directory of its own.

### Time limit

As on LeetCode, a solution that is correct but too slow fails with Time Limit Exceeded. hc judges speed against the problem's reference solution in your language, timed on your machine ([the protocol's time limit](protocol.md#time-limit)): a run may take **40 times** what the reference takes on the same cases, in total, with the reference's time counted as at least **1.25ms**, so the limit is never under 50ms: below that, times are mostly noise. Both values may still change. The run stops at the case during which your total passes the limit: that case is Time Limit Exceeded (it is ended even if it would never return), and the cases after it are Not Run. Such a run has no total time, in the result or in the history (where its TIME is `—`): it would count only the cases that ran, so it compares with no other run. The same goes for a run that a crash stops.

- Forty times leaves room for a different way of writing the same algorithm, such as a map where the reference uses an array, but not for a worse complexity on inputs as large as the problem's constraints allow. Whether a slower solution fails depends on the problem's cases: some problems cannot separate two complexities within their constraints, and then the slower one passes, as it would on LeetCode.
- The first run of a problem times the reference: three runs on each case, of which hc keeps the fastest, in `.hc/cache/baseline/<pack>/<problem>/<language>.json`. hc notes it on stderr ("Timing the Go reference…"), and later runs, with any selection of cases, reuse the times. They are measured again when hc, your toolchain, `problem.yaml` or the reference change.
- A problem without a reference in your language is not judged ([readiness](protocol.md#readiness)), so it has no time limit either. References, `hc cases build` and `hc validate --run` are never limited, and neither is `hc debug`. Their runs of a reference on generated cases, which compute the expected outputs, count as one of its three timed runs, except while `hc validate` or `hc pack check` checks several problems at once (`--jobs`).
- The `timeout` of `hc.yaml` is another limit, on the wall-clock time of the whole run: it guards against a solution that hangs. It can also come first: when the reference takes more than a fortieth of it on the run's cases (250ms with the default 10s, as Python references can on problems with many large cases), a much slower solution reaches the `timeout` before the time limit. The verdict is the same, Time Limit Exceeded, only after the whole `timeout`.

## Getting problems

### From LeetCode

```sh
hc fetch 3 146 lru-cache    # by number, slug or URL, from leetcode.com
hc fetch --site cn 1        # from leetcode.cn, which also has Chinese statements
hc fetch https://leetcode.cn/problems/two-sum/description/   # any problem URL; the URL picks the site
hc fetch --site cn "LCR 001"                                  # leetcode.cn IDs such as LCR/LCP
hc gen 146                  # make it ready: test cases and the judge rule
```

`hc fetch` downloads the statement and its images, the signature, the Go and Python starter code, and the examples. Expected outputs are included when they can be parsed from the statement.

It writes the statements in the locales of the pack it fetches into: `locales` in `hc.yaml` for your own problems (see [statements in several languages](#statements-in-several-languages)). Both sites have the English statement; only leetcode.cn has the Chinese one, and its title:

| `locales` in `hc.yaml` | leetcode.com | leetcode.cn |
|---|---|---|
| `[en]` (the default) | `description.md`: English | `description.md`: English |
| `[en, zh-CN]` | `description.md`: English; no Chinese statement, and a note says so | both: English in `description.md`, Chinese in `description.zh-CN.md`, both titles |
| `[zh-CN]` | refused: leetcode.com has no Chinese statement | `description.md`: Chinese |

Fetching a problem again from the other site adds the statements it lacks, without `--force` and without touching anything else: `hc fetch two-sum` then `hc fetch --site cn two-sum` gives a problem both statements. The statements share `assets/`, and an image both use is stored once. A problem that exists only in Chinese (LCR, LCP) cannot be fetched into a pack whose default locale is `en`.

A fetched problem is **not ready to submit** yet. LeetCode only publishes the examples, and not how answers are judged: whether order matters, whether several answers are valid. Until that is decided, `hc test` shows your output next to the example's answer without a verdict, and `hc submit` refuses. `hc gen` writes the full test suite and decides the judge rule from the statement, writing a checker when no built-in rule fits; you can also set `judge.compare` yourself (see the protocol's [compare modes](protocol.md#compare-modes)). The list marks such problems with `?` in the status column, where solved problems show `✔` and attempted ones `●`.

A problem is judged only in the languages it has a reference solution in: a Go solution needs `reference/solution.go`, a Python one `reference/solution.py` (the protocol's [readiness](protocol.md#readiness)). Without one, `hc test` shows your output without a verdict, `hc submit` refuses and names the missing file, and the list shows `?` for the language you solve in. `hc gen` writes a reference in every language the pack is solved in.

Premium problems are not available. Neither are problems whose signature uses types hc does not support yet (such as `MountainArray` in interactive problems); `hc fetch` says so.

### Your own problems

```sh
hc new my-problem --kind function   # scaffold a problem (--kind class for a design problem)
$EDITOR ~/handcoding/packs/local/problems/my-problem/problem.yaml
hc validate my-problem              # check the signature and the cases as you go
hc gen my-problem                   # have an agent write a reference and cases, or write them yourself:
hc cases build my-problem           #   expected outputs from reference/solution.*
hc validate my-problem --run        #   every reference must pass every case
hc import ./my-problems/            # import problem directories shared by others, or a pack's problems
hc rm my-problem                    # remove a problem (--purge: your solutions and history of it too)
```

`hc fetch`, `hc new` and `hc import` write into `packs/local/problems/`. The format of a problem, from `problem.yaml` to the reference solution, is specified by the [problem protocol](protocol.md); `hc new` scaffolds one to fill in, and hc's regression set ([`testdata/regression`](../testdata/regression) in its repository) has complete examples of every feature.

- `hc cases build` runs a reference solution: the Go one if there is one, else the Python one; `-l` picks one. It refuses problems judged with `any_of`, whose `expected` lists every acceptable answer and has to be written by hand.
- `hc validate` reports every break of the protocol's [validity rules](protocol.md#validity-rules) as an error, with the case number and field. With `--run` it also runs every reference solution on every case, and through the checker if there is one. It warns about an undecided judge rule (no `judge.compare`), no cases, cases without `expected`, generated cases without a digest, stored cases over 100 KB, no case marked as an example, no reference solution or none in a language the pack is solved in, and statement images that do not exist. `hc test`, `hc submit` and `hc debug` refuse to run a case that breaks these rules, and name it.
- `hc import` copies only the entries the protocol defines (anything else, such as editor files or a `solutions/` folder, is left behind), and refuses problems whose slug or ID is already among your own problems (`--force` replaces a problem with the same slug). Slugs and IDs only need to be unique within a pack, so importing a problem that an installed pack also has is fine. It also refuses a problem whose statements and titles do not match the locales of your own problems, as `hc validate` would report them: one without a statement in each of them, or with one in another locale.
- `hc rm` deletes a problem's directory, and in a pack checkout takes its slug out of the groups of `pack.yaml`, keeping the file's comments and layout. It asks first (`--yes` skips the question) and refuses installed packs. Your solutions, submission history and star of the problem are kept, so a problem with the same slug added later gets them back; `--purge` deletes them as well.
- `hc validate` without arguments also checks every installed pack's `pack.yaml`, and that its groups name problems of the pack. Each problem's directory name, slug and ID are checked against the rest of its pack.

hc carries the protocol (in English) for its own version, so it is at hand offline:

```sh
hc docs protocol                    # the whole protocol
hc docs protocol --list             # its sections
hc docs protocol structure-inputs   # one section, with its subsections
```

A section is named by its heading's anchor on GitHub, as in `protocol.md#structure-inputs`. On a terminal the output is rendered like a statement; piped, or with `--raw`, it is Markdown.

### Statements in several languages

A pack can give its statements in several languages, its [locales](protocol.md#locales): `locales: [en, zh-CN]` in `pack.yaml`. The first is the pack's default, the language of `description.md` and `title`; the others are in `description.<locale>.md` and `titles` in `problem.yaml`. The pack's author decides which locales it has and which comes first. For your own problems you are the author, and `locales` in `hc.yaml` sets them, `[en]` when unset:

```yaml
locales: [en, zh-CN]   # your own problems: English first, Chinese too
```

- `i` in the list switches a pack's locale, and hc remembers the choice for each pack in `.hc/`. A pack you open for the first time shows its default locale.
- `hc show <problem> --locale zh-CN` shows a statement in a locale; without `--locale`, it shows the one you chose last in the list.
- When a problem lacks the chosen locale, which only happens in a pack that does not validate, hc shows the default one and says so.
- `hc validate` reports a problem without a statement or a title in one of the pack's locales, and statements or titles in a locale the pack does not declare. `hc pack check` fails on them too.
- hc translates nothing. `hc fetch` writes the statements LeetCode has (see [above](#from-leetcode)); for your own problems, you write each locale's statement yourself. `hc gen` and `hc fix` read only the statement in the default locale, and never write one.

### Test cases

```sh
hc cases list 3                     # show the cases
hc cases add 3 -i '{"s":"abba"}'    # append a case (expected output from the reference, if any)
hc cases add 3 --generate long --args '{"n":100000}'   # append a generated case
hc cases edit 3                     # edit cases.jsonl in your editor
hc cases build 3                    # fill in missing expected outputs from the reference (--force: all)
```

### Generated cases

A large input, the kind that catches a solution of the wrong complexity, does not belong in `cases.jsonl`: a few of them make a problem megabytes large, and every change to them stays in the pack's git history. Write a [generator](protocol.md#generators) instead, a function of the problem's `generator.py` that returns the input, and add a case that calls it:

```python
# generator.py
def descending(rng, n):
    return {"nums": list(range(n, 0, -1))}      # a hand-made worst case: rng unused

def distinct(rng, n):
    return {"nums": rng.sample(range(-10**9, 10**9 + 1), n)}
```

```json
{"generate":{"name":"descending","args":{"n":100000}},"note":"descending, quadratic for insertion sort"}
{"generate":{"name":"distinct","args":{"n":100000},"seed":7}}
```

- `hc cases build` runs the generators afresh and writes each case's `digest`, the hash of the input it built. On the cases whose digest is new or changed (all of them with `--force`), it judges every other reference against the outputs of the one it runs, so the Go and the Python reference must agree. A generated case stores no expected output.
- `hc test`, `hc submit` and `hc debug` build the generated cases they run: Python runs the generators, whatever your solution's language, and the reference solution in your language (else another) computes the expected outputs, which are then judged as usual. The results are kept in `.hc/cache/cases/<pack>/<problem>/<language>.json` and built again when hc, `problem.yaml`, `generator.py`, the reference or the generated cases change.
- A generator that fails, builds an input other than its digest's, or an invalid one, and a reference that fails on a generated input, are errors of the problem, not verdicts on your solution: the run stops and names the file and the case. `hc validate --run` and `hc pack check` build the cases afresh, never from the cache, so that they catch a generator that is not deterministic.
- `hc test --generate <name> --args '{…}' --seed N` builds a custom input with a function of `generator.py`, without adding a case to `cases.jsonl` or touching the cache, and runs it like `-i`; the reference computes its expected output. Timing a solution on `n` and on `n/2` this way shows how its time grows.
- `hc cases list` shows a generated case as its call, `distinct(n=100000) seed 7`, and a failing one in a report as the call followed by the clipped input. `hc cases list --expand` runs `generator.py` and shows the inputs it builds now (with `--json`, as the lines they would be if stored, for a script to check), without checking the digests.
- `hc validate` warns about a stored case over 100 KB, and `hc pack check` fails on it.

## Packs

A [pack](protocol.md#packs) is a directory of ready-made problems with a `pack.yaml`, usually a git repository. hc runs the `git` on your PATH to install and update packs.

```sh
hc pack install https://github.com/MorePeanuts/leetcode.hc.git   # clone a pack into packs/<name>/
hc pack install ../my-pack --ref v1.2.0                          # a local git repository, at a tag or branch
hc pack list                                # packs, versions, commits, sources and your progress
hc pack update                              # update every pack (or name some)
hc pack update leetcode --ref main          # follow another tag or branch
hc pack remove leetcode                     # delete the pack; your solutions and history stay
hc pack remove leetcode --purge             # ... and delete them too
```

> [!WARNING]
> Installing a pack means trusting its code: its reference solutions, checkers and generators run on your machine when you test, submit or validate. Install packs only from sources you trust.

- **`hc pack install <source>`** takes a git URL, or a directory. A git URL is cloned partially (`--filter=blob:none`): hc downloads every commit, but only the files of the commits it checks out, so a pack's history costs little however large its files once were. A directory that is a git repository is cloned whole; any other directory holding `pack.yaml` is copied (its `pack.yaml` and `problems/` only). `--ref` installs a tag or branch of a git source instead of its default branch. The pack goes into `packs/<name>/`, named after its `pack.yaml`. hc refuses a `pack.yaml` that is invalid or written for a protocol it does not read (and says whether to upgrade hc or the pack), and a name that another pack in `packs/` has. Problems that fail to load are reported, but do not keep the pack from being installed.
- **`hc pack list`** shows each pack's name, version, number of problems, progress (problems with an accepted submission), installed commit and source. A directory of `packs/` that `hc.yaml` does not list, such as one copied by hand, is *unmanaged*; a pack that `hc.yaml` lists but `packs/` lacks is *missing*.
- **`hc pack update [name]...`** fetches each pack and checks out the latest commit of its ref: a branch moves on, a tag stays put unless it was moved upstream, and `--ref` switches to another one. A pack installed without `--ref` follows the source's default branch, also when the source renames it. A copied pack is copied again. Installed packs are read-only, so a pack with local changes is refused; `--force` discards them. An update whose `pack.yaml` hc cannot read, or that renames the pack, is undone. Afterwards hc lists the problems added and removed. Your solutions and history are keyed by the problem's directory and slug, so those of a removed problem, or of one whose directory was renamed, are kept but no problem uses them: hc says so, and deletes nothing.
- **`hc pack remove <name>`** deletes `packs/<name>/` and its entry in `hc.yaml`. Your solutions (`solutions/<name>/`) and its history stay, so installing the pack again brings them back; `--purge` deletes them too, after asking (`--yes` skips the question). `local` cannot be removed.
- `hc pack install`, `update` and `remove` run one at a time in a workspace: while one runs, another refuses to start and says so, rather than undo its changes to `packs/` and `hc.yaml`.

### The packs list

hc records every pack it installs in `hc.yaml`, and updates the entry on every `hc pack update`:

```yaml
packs:
  - name: leetcode
    source: https://github.com/MorePeanuts/leetcode.hc.git
    ref: v0.3.0        # the tag or branch followed; absent: the default branch
    commit: 3f2a9c1…   # the commit installed
  - name: mine
    source: /home/me/my-pack   # a copied directory: no commit
```

hc edits only this list, so the rest of `hc.yaml` keeps its comments and order. A local source is recorded as an absolute path.

To **reproduce a workspace** elsewhere, copy its `hc.yaml` into a new workspace (or keep the workspace in git: the `.gitignore` of `hc init` leaves the installed packs out) and run `hc pack install` with no argument: it installs every listed pack that is missing, at its recorded commit, and copies copied directories again.

### Checking a pack

`hc pack check` is what a pack's CI runs before publishing. Run it in a pack checkout ([working on a pack](#working-on-a-pack)), or pass its directory: `hc pack check ./my-pack`. It checks that `pack.yaml` is valid and of a supported protocol, that its groups name problems of the pack, and that every problem loads, is valid, has a canonical directory name and a unique slug and ID, is ready (`judge.compare` is set), has cases that all have an expected output (or, generated, a digest) and none over 100 KB, a reference solution in every language of the pack's `languages`, and passes `hc validate --run`: every reference solution passes every case, generated ones built afresh. So it needs Go and Python 3.12+, and a reference that cannot be run is an error. It prints one line per problem and a summary, and exits non-zero on any failure.

`hc pack check` and `hc validate` check several problems at once: `--jobs` (`-j`) of them, by default one per CPU and at most 4. The lines come in the same order as one at a time, each as soon as its problem and those before it are done, and the output of a problem's reference runs comes just before its line. Reference runs made at once are not timed for [time limits](#time-limit), since they compete for the machine; `-j 1` checks one problem after another and times them as before.

## Test cases from a coding agent

```sh
hc gen 3                                                   # Claude Code by default
hc gen 3 --agent codex -n 40 -m "more strings of one repeated character"
hc fix two-sum -c 5 -m "answers in a different order should pass"
```

`hc` runs the agent headless in the workspace (`claude -p` or `codex exec`). Before it starts, it installs or updates the workspace's skills (see [Agent skills](#agent-skills)); the agent follows **`hc-cases`**, and **`hc-problem`** for when the problem's definition itself is wrong. Following the skill, the agent:

1. writes a reference solution in every language the pack is solved in (`reference/solution.py`, `reference/solution.go`), and a brute-force one to cross-check when that helps;
2. writes test **inputs** to `cases.jsonl`, covering edge cases and special structures, and large inputs as [generated cases](#generated-cases);
3. runs `hc cases build`, so expected outputs come from running the reference, not from the model;
4. runs `hc validate --run` until every reference passes.

The presets run the agent in a sandbox where its commands can write only to the workspace and temporary directories (Claude Code's sandbox, Codex's `workspace-write`). So hc sets up the Python environment in `.hc/venv` before the agent starts, and gives the agent, preset or not, caches inside the workspace: `GOCACHE` is `.hc/cache/go-build` and `UV_CACHE_DIR` is `.hc/cache/uv`. The agent's first Go build is slower for it.

While the agent works, hc shows each command it runs and file it writes, then its final report, how long it ran (with Claude Code, its turns and cost) and where its trace is. **Every run is logged** in `.hc/agent/<pack>/<slug>/<time>-<gen|fix>.jsonl`, the newest 10 per problem. A log is JSON lines:

- the first is `{"type": "start", ...}`: hc's version, the task, the problem, the agent and its command line, and the prompt;
- then one per line the agent wrote, with the time hc read it: `{"type": "stdout", "event": ...}` holds an event of the presets' JSON output (Claude Code's `--output-format stream-json`, `codex exec --json`), and `"text"` any other line, of standard error (`"stderr"`) too;
- the last is `{"type": "end", ...}`: how long the run took, the exit code, and the error, if any.

Read it to see what the agent did and where its time went, for example with `jq`. The agents keep their own records too, but they are elsewhere and do not say which hc run they belong to.

To use another agent, set its command line in `hc.yaml`; `{prompt}` is replaced by the prompt:

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

The prompt points the agent at the skills in `.agents/skills/`, or in `.claude/skills/` when the command is `claude`. hc shows and logs such a command's output as text.

### Agent skills

hc ships skills that teach coding agents its problem protocol and workflows:

| Skill | Covers |
|---|---|
| `hc-problem` | what a problem is: its signature, statement and templates, and what to check after `hc fetch` |
| `hc-cases` | how it is judged: judge rules, reference solutions, checkers and test cases |
| `hc-pack` | maintaining a pack: landing problems, groups, stable slugs, protocol versions, CI and releases |

Which of them a workspace gets depends on its kind: a practice workspace gets `hc-problem` and `hc-cases`, a pack checkout ([working on a pack](#working-on-a-pack)) all three. They are installed into `.claude/skills/` for Claude Code and `.agents/skills/` for Codex, and listed in a block of `AGENTS.md`, which also tells agents to keep throwaway files in `.hc/scratch/<slug>/`, a directory per problem, so that agents working on different problems at once (several `hc gen` runs, say) do not overwrite each other's: by `hc init` in a new workspace, and by `hc gen`, `hc fix` and `hc skill install` in any; a practice workspace is also refreshed when a newer hc opens it (see below). In a practice workspace that block is a guide for any agent you open there to help you practice, with the skills as one section of it (it is written without them too): the workspace's layout, the hc commands to use (`hc show`, `hc test`; `hc submit` only when you ask, and the interactive `hc debug` never), and to teach rather than solve: hints before a solution, the pack's `reference/` kept as the answer key, your solution file changed only when you ask, installed packs never edited. In a pack checkout the block is the skill index alone, after the pack's own text. Claude Code does not read `AGENTS.md` itself, so hc also creates a `CLAUDE.md` that imports it (`@AGENTS.md`) when the workspace has none; one you have is yours, and hc never touches it. Because they live in the workspace, any Claude Code or Codex session started there can use them: ask it to "write an hc problem for ...", "generate cases for this hc problem" or "add this problem to the pack". The skills cite the sections of the protocol (`hc docs protocol <section>`) rather than repeating it.

```sh
hc skill list               # the skills this workspace gets; each one installed, up to date, outdated or modified, and by which hc version
hc skill install            # install or update the workspace's skills, and recreate AGENTS.md's block and CLAUDE.md
hc skill install --force    # also replace or remove the skills you edited
hc skill install hc-pack    # install a skill by name, whether the workspace gets it or not
hc skill show hc-cases      # print a skill
```

hc marks every copy it installs, with its version and a hash of the content (`.hc-skill.json` in the skill's directory), and replaces or removes only those: a skill you edited is kept unless you pass `--force`, and skills hc did not install are never touched. In `AGENTS.md`, hc rewrites only the block between `<!-- hc:begin -->` and `<!-- hc:end -->`; the rest of the file is yours, so write your own notes for agents outside the block. `hc skill install` without names brings the workspace to exactly its set: it also removes the skills hc installed that the workspace does not get (a skill installed by name included), and warns about edited skills instead of replacing or removing them.

**After upgrading hc**, a practice workspace keeps up on its own: the first command that opens it (the TUI included) sees that a skill hc installed, or its block of `AGENTS.md` or of `.gitignore`, is not what this hc writes, refreshes them, and says so in one line on stderr, such as `updated the workspace for hc v0.1.0-alpha.11: hc-cases, AGENTS.md`. It updates outdated skills, installs the skills of the set that are missing, removes the skills hc no longer ships and rewrites the block, but keeps a skill installed by name, and does not bring back a block of `AGENTS.md` or a `CLAUDE.md` you removed (`hc skill install` does). A skill you edited is kept, with a warning shown once. When everything is current it writes nothing. Only a release of hc does this, and only to skills from an older hc: a development build, or an older hc opening a workspace a newer one refreshed, leaves it as it is. `hc gen` and `hc fix` install missing or outdated skills of the set before they start the agent, in any version.

A pack checkout commits its skills, and its CI requires them to be those of the hc it pins ([CI and the pinned hc version](packs.md#ci-and-the-pinned-hc-version)), so hc never refreshes them there on its own: `hc gen` and `hc fix` only install the skills it lacks, and when its skills come from another hc than the one you run, `hc validate`, `hc pack check`, `hc gen` and `hc fix` say so, and that moving the pack to this hc is a step of its own (pin it in the workflow, `hc skill install`, commit).

## Debugging

```sh
hc debug two-sum            # Go: Delve, stopped at your function, on the first failing case
hc debug lru -l py -c 2     # Python: pdb, on case 2
```

Without `-c` or `-i`, `hc debug` debugs the first case your code fails. While your code and the problem are as they were at your last `hc test` or `hc submit` (in the TUI too), it takes that run's first failure without running again. Otherwise, or when that run passed only some of the cases, it first runs every case, then debugs the first that fails. With no failure it debugs case 1. `hc history restore` counts as a run of the restored code, since a submission ran every case. A run that stopped early, at a time limit or a crash, fails on the case it stopped at.

### Neovim (nvim-dap)

```sh
hc debug two-sum -c 2 --headless    # Go: dlv headless on 127.0.0.1:38697
hc debug lru -l py --headless       # Python: debugpy, from .hc/venv
```

```lua
local dap = require("dap")
dap.adapters.hc = { type = "server", host = "127.0.0.1", port = 38697 }
table.insert(dap.configurations.go, { type = "hc", name = "hc attach", request = "attach", mode = "remote" })
table.insert(dap.configurations.python, { type = "hc", name = "hc attach", request = "attach" })
```

Set breakpoints in your solution file, start `hc debug --headless`, then pick "hc attach" in Neovim.

## Running on LeetCode

`hc leetcode` runs a solution on LeetCode, or submits it to your own account, as the Run and Submit buttons of the problem's page do. It works on problems that come from LeetCode (fetched with `hc fetch`, or from a pack such as the official one): the site and slug are those of `source` in `problem.yaml`, and `--site` picks the other site.

```sh
hc leetcode login --site cn          # once per site: authorize hc in the browser
hc leetcode status                   # which sites hc is logged in to, and as whom
hc leetcode test two-sum             # run on LeetCode's example testcases
hc leetcode test two-sum -i $'[3,3]\n6'   # or on your own, in LeetCode's format
hc leetcode submit two-sum -l python # submit to your account
hc leetcode logout                   # forget every site's session (--site: one)
```

**Logging in.** LeetCode signs in with a CAPTCHA, so hc logs in with the session of a browser instead of a password. By default `hc leetcode login` opens the site's authorization page in your browser, as LeetCode's VS Code extension does (sign in there if you are not yet). LeetCode then sends the browser's cookies to hc on a port of `127.0.0.1`, and the page says when hc is logged in. The cookies travel in that URL, so it stays in the browser's history. On a machine without a desktop, or over SSH, open the page hc prints on your own machine with the port forwarded:

```sh
ssh -L 8765:127.0.0.1:8765 server     # on your machine
hc leetcode login --port 8765         # on the server
```

The authorization page is not a documented API of LeetCode's. If it stops working, log in with `--paste` (also used when standard input is not a terminal): open the site signed in and open the developer tools (F12), then copy one of these:

- the value of the `LEETCODE_SESSION` cookie, under Application (Chrome, Edge) or Storage (Firefox, Safari) → Cookies;
- or, under Network, any request to the site (reload the page if the list is empty): right-click → Copy → Copy as cURL, or its `Cookie` request header.

Paste it when `hc leetcode login --paste` asks (nothing is shown as you paste), or pipe it in (`pbpaste | hc leetcode login --site cn` on macOS). hc finds `LEETCODE_SESSION` in what you paste.

Either way, hc checks the session with LeetCode and saves it, readable by you only, in `hc/leetcode.json` of your user configuration directory (`~/Library/Application Support` on macOS, `~/.config` on Linux), never in the workspace. A session lasts a few weeks; when LeetCode refuses one, log in again. `hc leetcode status` checks each saved session with LeetCode and shows the user it signs in as, or that LeetCode no longer accepts it; its exit status is 1 unless hc is logged in (to `--site`, or to some site) and every saved session works. `hc leetcode logout` forgets the sessions of every site, or of `--site`, but only in hc: LeetCode keeps a session valid until it expires, so log out in the browser to end it there.

**Code.** hc sends the solution `hc test` would run (`-l` as there), turned into LeetCode's code: Python drops its imports from `hc_runtime` (LeetCode defines `ListNode`, `TreeNode` and `Node` itself), and Go its `package main`. A Python function problem must be a method of `class Solution` there; hc also accepts a module-level function, LeetCode does not.

**Results.** `test` shows each example's output next to LeetCode's expected answer and what it printed. `submit` shows the verdict, the testcases passed, and the runtime and memory with their percentiles. When a submission fails, it shows the case it failed on, your output and the expected one; when the problem's inputs are LeetCode's as they are (a function problem without [structure inputs](protocol.md#structure-inputs) or LeetCode's own types), it also prints the `hc cases add` command that adds the case, so that a case LeetCode has and the problem lacks can join its tests. LeetCode results are not recorded in hc's history.

## Configuration

`hc.yaml` at the workspace root:

```yaml
language: go          # default solution language: go | python
editor: nvim          # falls back to $VISUAL, $EDITOR, then vi
theme: auto           # statement colors: auto | dark | light
images: auto          # statement images: auto | kitty | blocks | none
timeout: 10s          # wall-clock limit of a run, its checker and its generators (protocol: run-limit)
python:
  interpreter: python3.12  # Python 3.12+ to use instead of .hc/venv; e.g. a full path. Unset by default
agent:
  name: claude        # claude | codex, or set command (see above)
leetcode:
  site: com           # com | cn
locales: [en]         # the statement languages of your own problems, the default first
```

`images: auto` picks `kitty` in kitty, Ghostty and [herdr](https://herdr.dev), `blocks` in other truecolor terminals, and `none` otherwise.

- **herdr** draws kitty graphics itself, so images show whenever the terminal around herdr supports kitty graphics: kitty, Ghostty, or WezTerm with `enable_kitty_graphics = true`. In other terminals the image area stays blank; set `images: blocks`, and press `o` to see the original.
- **tmux**: auto never picks kitty. If the outer terminal supports Unicode placeholders (kitty, Ghostty), set `images: kitty` and enable tmux's `allow-passthrough`.
- **WezTerm** supports kitty graphics but not Unicode placeholders, so `images: kitty` shows garbled characters there. Keep `blocks` when running WezTerm directly.

## Current limitations

- Interactive problems aren't supported yet.
- `hc fetch` imports problems with [structure inputs](protocol.md#structure-inputs) (a list with a cycle, intersecting lists, node references such as Lowest Common Ancestor), working out the structures from the difference between LeetCode's metadata, which describes its test data, and its template, which is the real signature: test data the template does not take becomes a hidden input, a value the template takes as a node becomes a `ref`, a node parameter missing from the test data becomes a `copy`, a `pos` makes a `cycle`, 160's `skipA` and `skipB` make a `join`, and a returned node gets `returns_ref`. The examples are mapped onto the new inputs. Example outputs that are not plain JSON, such as 142's "tail connects to node index 1", are left to `hc gen`. Problems 141, 142, 160, 236, 237 and 1379 of the regression set show them.
- `hc fetch` imports problems with [LeetCode's `Node` types](protocol.md#leetcodes-node-types), `NestedInteger` and `Employee`, reading their signature from LeetCode's Go template. `returns_copy` (Clone Graph) is left to `hc gen`, and so are the examples of iterator problems such as Flatten Nested List Iterator, which LeetCode gives without the calls.
- Problems that encode a value and decode it back (297, 449, 535) are judged as [round trips](protocol.md#round-trip-problems): only the decoded value is compared, so requirements on the encoding itself (449: "as compact as possible") are not checked. `hc fetch` imports them, reading the encoder, the decoder and whether they share an instance from the harness in LeetCode's template comment. Problems 297 and 535 of the regression set show both kinds.
- Time is limited (see [Time limit](#time-limit)), but memory is not.
- Python code runs with a recursion limit of 10^6 rather than Python's default 1000, in a thread with a 512 MB stack (virtual memory, used only as deep as the code goes), so a plain recursive solution goes as deep as the inputs without `sys.setrecursionlimit`. Recursion through C code (a `functools.cache` function, comparisons made by `sorted` or `max`) goes as deep as that stack allows from Python 3.14, but Python 3.12 and 3.13 cap it at about 3000 calls whatever the limit. Under `hc debug`, the code runs in the main thread, whose stack is smaller.
- LeetCode premium problems cannot be fetched.
- Problems with many valid answers that cannot all be listed ("return any valid answer") and randomized problems are judged by a [checker](protocol.md#checkers), which `hc gen` writes; problems 5, 108, 210, 324, 384 and 528 of the regression set have one. Checkers are Python, so these problems need Python 3.12 or later even when you solve them in Go; the virtual environment hc manages provides it.
