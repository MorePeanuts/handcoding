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

`hc init` creates an empty workspace, by default `~/handcoding`:

```
~/handcoding/
  hc.yaml                                  configuration, and the list of installed packs
  packs/local/problems/0001-two-sum/       your own problems (see the problem protocol)
  packs/<name>/                            installed packs (read-only)
  solutions/local/go/0001-two-sum/         your code: solution.go, plus generated go.mod and zz_hc_driver.go
  solutions/local/python/0001-two-sum/     your code: solution.py, plus generated hc_runtime.py and _hc_runner.py
  .hc/hc.db                                submission history (SQLite)
  .hc/venv/                                Python virtual environment managed by hc
  .hc/cache/                               Go and uv caches for coding agents
  pyrightconfig.json                       points pyright at .hc/venv
```

`hc init --lang python` makes Python the default solution language (`language` in `hc.yaml`; default `go`) and creates the Python environment described below right away. If that fails, for example because neither uv nor Python 3.10+ is installed, `hc init` warns and creates the workspace anyway.

`hc` finds the workspace from `-w <dir>`, then `$HC_WORKSPACE`, then the nearest parent directory containing `hc.yaml` or `pack.yaml` (a pack checkout, see [working on a pack](#working-on-a-pack)), then `~/handcoding`. You can run it from anywhere.

### Packs in a workspace

The problems of a workspace come in [packs](protocol.md#packs), each a directory of `packs/`:

- `packs/local/` holds your own problems: what `hc fetch`, `hc new` and `hc import` add. It has no `pack.yaml` (`local` is a reserved name), and is solved in every language. Its statements are in the [locales](#statements-in-several-languages) that `locales` in `hc.yaml` sets, English (`[en]`) by default.
- Every other directory is an installed pack: a `pack.yaml` whose `name` is the directory's name, and its `problems/`. Installed packs are **read-only**: `hc gen`, `hc fix` and `hc cases add|rm|edit|build` refuse to change them, since updating the pack would replace the change. A pack whose `pack.yaml` lists `languages` can only be solved in those: `hc start -l` refuses the others, and if your default language is not among them, hc uses the pack's first language and says so. `hc pack install` installs them (see [Packs](#packs)); a pack copied into `packs/` by hand works too, and `hc pack list` shows it as unmanaged.

A pack that hc cannot read (no `pack.yaml`, an invalid one, or one written for a newer protocol, which says to upgrade hc) is reported by `hc validate` and the list, and its problems are left out; the rest of the workspace works.

Solutions live in `solutions/<pack>/<lang>/<dir>/`, and the history keeps each submission's pack and slug, so the same slug in two packs is two problems.

A workspace made by an older hc keeps its problems in `problems/` instead of `packs/`. hc stops there and says so: create a new workspace with `hc init <dir>` and move your problems into its `packs/local/problems/`. Solutions and history are not carried over.

Each Go problem is a standalone module, and the driver sits next to `solution.go`. So gopls, `go build` and Delve breakpoints all work on your real file. The generated files are rewritten before every run, so don't edit them. `go.mod` follows your Go: its `go` line is your toolchain's version (`go 1.27` for Go 1.27.2), so your solutions can use everything it supports. hc builds with `GOTOOLCHAIN=local` and never downloads another toolchain.

Python solutions run in a virtual environment that hc manages in `.hc/venv`. hc creates it the first time a command needs Python (`hc init --lang python`, `hc start`, `test`, `submit`, `debug`, `cases build`, `validate --run`):

1. with [uv](https://docs.astral.sh/uv/) if it is installed: `uv venv --python '>=3.10'`, which downloads a Python if none is installed;
2. otherwise with the newest Python 3.10+ on PATH (`python3`, `python` or `python3.N`) and `-m venv`. On Debian and Ubuntu this needs the `python3-venv` package.

The environment holds debugpy for headless debugging. Downloading Python and installing debugpy need network access, once; judging stays offline. If the environment stops working (the workspace was moved, or its Python removed), hc creates it again. `hc env sync` recreates it on demand, for example after installing uv. hc also writes `pyrightconfig.json` at the workspace root, unless you have one, so pyright and basedpyright check your solutions with the same interpreter. To use your own Python instead (conda, a custom build), set `python.interpreter` in `hc.yaml`; hc then creates no environment.

The oldest supported toolchains are Python 3.10 and Go 1.22 (the protocol's [language versions](protocol.md#language-versions)). Your own solutions may use anything your toolchain supports: `hc test`, `hc submit` and `hc debug` name the toolchain they run with (`[Python 3.12.9]`) and only warn when it is older. Code that problems ship must work on the oldest versions, so `hc cases build` and `hc validate --run` refuse to run reference solutions on an older toolchain. A newer one is fine: `hc validate --run` (and so `hc pack check`) also checks every Go reference against Go 1.22, by vetting it in a module whose go directive is `go 1.22`, which reports newer language features and standard library symbols.

Consider running `git init` in the workspace to keep a record of your practice. The `.gitignore` that `hc init` writes leaves out `.hc/` and the installed packs, which `hc pack install` reinstalls from `hc.yaml`.

### Working on a pack

To start a pack of your own, run `hc pack init`: it writes `pack.yaml`, a `.gitignore`, a CI workflow and the agent files, ready to commit. [Writing packs](packs.md) is the guide to creating, checking and publishing one.

A pack checkout, that is a directory with `pack.yaml` such as a clone of a pack's repository, is a workspace as it is: run hc in it or in any directory below it, or point `-w` at it. It needs no `hc init`, and `hc init` refuses a directory inside a pack. In this authoring workspace the pack is the only pack, and it is writable: `hc fetch`, `hc new` and `hc import` add problems to its `problems/`, and `hc gen`, `hc fix` and `hc cases` change them. So the authoring flow needs no flags: `hc fetch two-sum`, `hc gen two-sum`, `hc validate --run`, and `hc pack check` before publishing (see [checking a pack](#checking-a-pack)). There is no `local` pack, a `packs/` directory in the checkout is ignored, and problems are named by their slug alone.

```
my-pack/
  pack.yaml  problems/                     the pack (see the problem protocol)
  hc.yaml                                  optional: your settings (language, editor, agent, timeout, python)
  solutions/<pack>/<lang>/<dir>/           your solutions, as in a practice workspace
  .hc/  pyrightconfig.json                 hc's state, as in a practice workspace
  .claude/  .agents/  AGENTS.md            the agent skills, once a command installs them
```

Without `hc.yaml` hc uses the defaults, and it never creates one in a checkout. Its `locales` are those of `pack.yaml`, so an `hc.yaml` there must not set them, as it must not list `packs`. A `pack.yaml` that hc cannot read, for example one of a newer protocol, does not keep the workspace from opening: `hc validate` says what is wrong. What hc adds is yours, not the pack's, so keep it out of the repository's history:

```
# .gitignore
.hc/
solutions/
hc.yaml
pyrightconfig.json
```

`hc pack init` writes this `.gitignore`, and commits to keeping `.claude/`, `.agents/` and `AGENTS.md` in the repository, so that a fresh clone describes itself to agents. In a pack created otherwise, ignore them too, or commit them (hc changes only its own skills and its block of `AGENTS.md`).

A pack installed in a practice workspace (`packs/<name>/`, beside `hc.yaml`) is not a checkout: hc run inside it uses the enclosing workspace, where the pack stays read-only.

## The interactive list

Run `hc` (or `hc list`) to open the problem list. It works best with the terminal split in two: `hc` on one side, your editor on the other.

- Press `enter` on a problem to open it. The statement is shown, the solution file is created from the template, and its absolute path is **copied to the clipboard**. In the editor, `:e <paste>` and start writing. To read a statement without starting the problem, press `v` instead: nothing is created or copied until an action such as `e`, `t` or `y` needs the solution.
- Test, submit, debug and history are all available from the statement. Results appear in the TUI, with your program's own output listed separately below them.
- The debugger takes over the terminal. The screen is cleared before it starts, and the TUI comes back when it exits. If the last run had a failing case, `d` debugs that case.
- Statement images are drawn in the terminal. kitty and Ghostty show the original image. Other truecolor terminals show a block-character approximation that loses detail; press `o` to open the original in your image viewer. See [`images`](#configuration) to choose the mode.
- Statement colors follow the terminal background. If they look wrong, set `theme` in `hc.yaml`.
- The list shows one pack at a time. With more than one pack, a tab bar at the top names each pack with its progress (solved/total), and `tab` / `shift+tab` switch packs; on a narrow terminal the bar scrolls to keep the current pack in view. A pack without problems, such as `local` before you add one, gets no tab, unless no pack has problems yet.
- `c` chooses a group of the pack, or all of its problems, and shows the progress of each. A group lists its problems in the group's order. Search and the filter apply to the pack and group shown.
- With more than one pack that has problems, `hc` opens on a pack picker that lists them with their progress, the pack you were in last under the cursor. `enter` opens a pack's list, and `esc` on the list comes back to the picker. With one such pack, `hc` opens its list directly.
- `hc --pack <name>` (or `hc list --pack <name>`) skips the picker and opens that pack, even an empty one, and `--group <name>` that group (a group name that only one pack uses needs no `--pack`). Each pack opens on the group you chose last.
- `i` switches the statement language of a pack whose statements come in [several languages](#statements-in-several-languages): titles in the list, and the statement on the detail screen. The list header and the statement's title line show the one in use.

| Key | Action | Key | Action |
|---|---|---|---|
| `enter` | Open the problem (and copy the solution path) | `e` | Open the solution in your editor |
| `t` | Test on the examples | `s` | Submit (all cases) |
| `d` | Debug (the last failing case, if any) | `h` | Submission history: code, judge details, restore, delete |
| `y` | Copy the solution path again | `L` | Switch language |
| `r` | Reset to the template | `D` | Discard: delete the solution and its history, back to not started |
| `/` | Search | `f` | Filter: all / unsolved / solved |
| `tab` / `shift+tab` | Next / previous pack | `c` | Choose a group of the pack |
| `v` | View the statement only | `o` | Open the statement's images (statement view) |
| `i` | Switch the statement language | `esc` / `q` | Back (`esc` on the list: to the pack picker); `q` on the list quits |
| `ctrl+c` | Quit | | |

## Commands

```
Practice       list  start  edit  show  test  submit  debug  reset  discard  history
Problems       init  fetch  new  import  cases  validate  pack  docs
Coding agent   gen  fix  skill
```

Problems can be referred to by slug, slug prefix, number or directory name: `hc test 1`, `hc test two`. These search every pack. `<pack>/<ref>` searches one: `hc test local/two-sum`, `hc test leetcode/1`. When a slug (or number) is in several packs, a bare reference is an error that lists them as `pack/slug`, and a prefix must match one slug across all packs.

```sh
hc start two-sum -e                                 # create the solution and open the editor
hc test two-sum                                     # run the examples (or the first 3 cases if none is marked)
hc test two-sum -c 4 -c 5                           # run chosen cases
hc test two-sum --all -v                            # all cases, full output, not recorded
hc test two-sum -i '{"nums":[1,2,3],"target":5}'    # custom input (expected output from the reference, if any)
hc test lru-cache -l py                             # solve in Python
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
```

`hc fetch`, `hc new` and `hc import` write into `packs/local/problems/`. The format of a problem, from `problem.yaml` to the reference solution, is specified by the [problem protocol](protocol.md); `hc new` scaffolds one to fill in, and hc's regression set ([`testdata/regression`](../testdata/regression) in its repository) has complete examples of every feature.

- `hc cases build` runs a reference solution: the Go one if there is one, else the Python one; `-l` picks one. It refuses problems judged with `any_of`, whose `expected` lists every acceptable answer and has to be written by hand.
- `hc validate` reports every break of the protocol's [validity rules](protocol.md#validity-rules) as an error, with the case number and field. With `--run` it also runs every reference solution on every case, and through the checker if there is one. It warns about an undecided judge rule (no `judge.compare`), no cases, cases without `expected`, no case marked as an example, no reference solution, and statement images that do not exist. `hc test`, `hc submit` and `hc debug` refuse to run a case that breaks these rules, and name it.
- `hc import` copies only the entries the protocol defines (anything else, such as editor files or a `solutions/` folder, is left behind), and refuses problems whose slug or ID is already among your own problems (`--force` replaces a problem with the same slug). Slugs and IDs only need to be unique within a pack, so importing a problem that an installed pack also has is fine. It also refuses a problem whose statements and titles do not match the locales of your own problems, as `hc validate` would report them: one without a statement in each of them, or with one in another locale.
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
hc cases edit 3                     # edit cases.jsonl in your editor
hc cases build 3                    # fill in missing expected outputs from the reference (--force: all)
```

## Packs

A [pack](protocol.md#packs) is a directory of ready-made problems with a `pack.yaml`, usually a git repository. hc runs the `git` on your PATH to install and update packs.

```sh
hc pack install https://github.com/someone/handcoding-pack.git   # clone a pack into packs/<name>/
hc pack install ../my-pack --ref v1.2.0                          # a local git repository, at a tag or branch
hc pack list                                # packs, versions, commits, sources and your progress
hc pack update                              # update every pack (or name some)
hc pack update leetcode --ref main          # follow another tag or branch
hc pack remove leetcode                     # delete the pack; your solutions and history stay
hc pack remove leetcode --purge             # ... and delete them too
```

> [!WARNING]
> Installing a pack means trusting its code: its reference solutions and checkers run on your machine when you test, submit or validate. Install packs only from sources you trust.

- **`hc pack install <source>`** takes a git URL, or a directory. A directory that is a git repository is cloned like a URL; any other directory holding `pack.yaml` is copied (its `pack.yaml` and `problems/` only). `--ref` installs a tag or branch of a git source instead of its default branch. The pack goes into `packs/<name>/`, named after its `pack.yaml`. hc refuses a `pack.yaml` that is invalid or written for a protocol it does not read (and says whether to upgrade hc or the pack), and a name that another pack in `packs/` has. Problems that fail to load are reported, but do not keep the pack from being installed.
- **`hc pack list`** shows each pack's name, version, number of problems, progress (problems with an accepted submission), installed commit and source. A directory of `packs/` that `hc.yaml` does not list, such as one copied by hand, is *unmanaged*; a pack that `hc.yaml` lists but `packs/` lacks is *missing*.
- **`hc pack update [name]...`** fetches each pack and checks out the latest commit of its ref: a branch moves on, a tag stays put unless it was moved upstream, and `--ref` switches to another one. A copied pack is copied again. Installed packs are read-only, so a pack with local changes is refused; `--force` discards them. An update whose `pack.yaml` hc cannot read, or that renames the pack, is undone. Afterwards hc lists the problems added and removed. Your solutions and history are keyed by the problem's directory and slug, so those of a removed problem, or of one whose directory was renamed, are kept but no problem uses them: hc says so, and deletes nothing.
- **`hc pack remove <name>`** deletes `packs/<name>/` and its entry in `hc.yaml`. Your solutions (`solutions/<name>/`) and its history stay, so installing the pack again brings them back; `--purge` deletes them too, after asking (`--yes` skips the question). `local` cannot be removed.

### The packs list

hc records every pack it installs in `hc.yaml`, and updates the entry on every `hc pack update`:

```yaml
packs:
  - name: leetcode
    source: https://github.com/someone/handcoding-leetcode.git
    ref: v0.3.0        # the tag or branch followed; absent: the default branch
    commit: 3f2a9c1…   # the commit installed
  - name: mine
    source: /home/me/my-pack   # a copied directory: no commit
```

hc edits only this list, so the rest of `hc.yaml` keeps its comments and order. A local source is recorded as an absolute path.

To **reproduce a workspace** elsewhere, copy its `hc.yaml` into a new workspace (or keep the workspace in git: the `.gitignore` of `hc init` leaves the installed packs out) and run `hc pack install` with no argument: it installs every listed pack that is missing, at its recorded commit, and copies copied directories again.

### Checking a pack

`hc pack check` is what a pack's CI runs before publishing. Run it in a pack checkout ([working on a pack](#working-on-a-pack)), or pass its directory: `hc pack check ./my-pack`. It checks that `pack.yaml` is valid and of a supported protocol, that its groups name problems of the pack, and that every problem loads, is valid, has a canonical directory name and a unique slug and ID, is ready (`judge.compare` is set), has cases that all have an expected output and a reference solution, and passes `hc validate --run`: every reference solution passes every case. So it needs Go and Python 3.10+, and a reference that cannot be run is an error. It prints one line per problem and a summary, and exits non-zero on any failure.

## Test cases from a coding agent

```sh
hc gen 3                                                   # Claude Code by default
hc gen 3 --agent codex -n 40 -m "more strings of one repeated character"
hc fix two-sum -c 5 -m "answers in a different order should pass"
```

`hc` runs the agent headless in the workspace (`claude -p` or `codex exec`). Before it starts, it installs or updates the workspace's skills (see [Agent skills](#agent-skills)); the agent follows **`hc-cases`**, and **`hc-problem`** for when the problem's definition itself is wrong. Following the skill, the agent:

1. writes a reference solution, `reference/solution.py`, and a brute-force one to cross-check when that helps;
2. writes test **inputs** to `cases.jsonl`, covering edge cases, special structures and large inputs;
3. runs `hc cases build`, so expected outputs come from running the reference, not from the model;
4. runs `hc validate --run` until every reference passes.

The presets run the agent in a sandbox where its commands can write only to the workspace and temporary directories (Claude Code's sandbox, Codex's `workspace-write`). So hc sets up the Python environment in `.hc/venv` before the agent starts, and gives the agent, preset or not, caches inside the workspace: `GOCACHE` is `.hc/cache/go-build` and `UV_CACHE_DIR` is `.hc/cache/uv`. The agent's first Go build is slower for it.

To use another agent, set its command line in `hc.yaml`; `{prompt}` is replaced by the prompt:

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

The prompt points the agent at the skills in `.agents/skills/`, or in `.claude/skills/` when the command is `claude`.

### Agent skills

hc ships skills that teach coding agents its problem protocol and workflows:

| Skill | Covers |
|---|---|
| `hc-problem` | what a problem is: its signature, statement and templates, and what to check after `hc fetch` |
| `hc-cases` | how it is judged: judge rules, reference solutions, checkers and test cases |
| `hc-pack` | maintaining a pack: landing problems, groups, stable slugs, protocol versions, CI and releases |

Which of them a workspace gets depends on its kind: a practice workspace gets `hc-problem` and `hc-cases`, a pack checkout ([working on a pack](#working-on-a-pack)) all three. They are installed into `.claude/skills/` for Claude Code and `.agents/skills/` for Codex, and listed in a block of `AGENTS.md`: by `hc init` in a new workspace, and by `hc gen`, `hc fix` and `hc skill install` in any. Because they live in the workspace, any Claude Code or Codex session started there can use them: ask it to "write an hc problem for ...", "generate cases for this hc problem" or "add this problem to the pack". The skills cite the sections of the protocol (`hc docs protocol <section>`) rather than repeating it.

```sh
hc skill list               # the skills this workspace gets; each one installed, up to date, outdated or modified, and by which hc version
hc skill install            # install or update the workspace's skills (after upgrading hc)
hc skill install --force    # also replace or remove the skills you edited
hc skill install hc-pack    # install a skill by name, whether the workspace gets it or not
hc skill show hc-cases      # print a skill
```

hc marks every copy it installs, with its version and a hash of the content (`.hc-skill.json` in the skill's directory), and replaces or removes only those: a skill you edited is kept unless you pass `--force`, and skills hc did not install are never touched. In `AGENTS.md`, hc rewrites only the block between `<!-- hc:begin -->` and `<!-- hc:end -->`; the rest of the file is yours. `hc gen` and `hc fix` install missing or outdated skills of the workspace's set before they start the agent. `hc skill install` without names brings the workspace to exactly its set: it also removes the skills hc installed that the workspace does not get (a skill installed by name included). Both warn about edited skills instead of replacing or removing them.

## Debugging

```sh
hc debug two-sum -c 2       # Go: Delve, stopped at your function
hc debug lru -l py          # Python: pdb
```

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

## Configuration

`hc.yaml` at the workspace root:

```yaml
language: go          # default solution language: go | python
editor: nvim          # falls back to $VISUAL, $EDITOR, then vi
theme: auto           # statement colors: auto | dark | light
images: auto          # statement images: auto | kitty | blocks | none
timeout: 10s          # wall-clock limit per run; a problem's judge.timeout overrides it
python:
  interpreter: python3.12  # Python 3.10+ to use instead of .hc/venv; e.g. a full path. Unset by default
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
- Run time is measured, but memory is not limited.
- LeetCode premium problems cannot be fetched.
- Problems with many valid answers that cannot all be listed ("return any valid answer") and randomized problems are judged by a [checker](protocol.md#checkers), which `hc gen` writes; problems 5, 108, 210, 324, 384 and 528 of the regression set have one. Checkers are Python, so these problems need Python 3.10 or later even when you solve them in Go; the virtual environment hc manages provides it.
