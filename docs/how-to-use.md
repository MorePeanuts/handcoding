# Using handcoding

English · [简体中文](how-to-use_zh.md)

- [Workspace](#workspace)
- [The interactive list](#the-interactive-list)
- [Commands](#commands)
- [Getting problems](#getting-problems)
- [Test cases from a coding agent](#test-cases-from-a-coding-agent)
- [Debugging](#debugging)
- [Configuration](#configuration)
- [Current limitations](#current-limitations)

## Workspace

`hc init` creates a workspace, by default `~/handcoding`:

```
~/handcoding/
  hc.yaml                     configuration
  problems/0001-two-sum/      problem bank (see the problem protocol)
  solutions/go/0001-two-sum/  your code: solution.go, plus generated go.mod and zz_hc_driver.go
  solutions/python/...        your code: solution.py, plus generated hc_runtime.py and _hc_runner.py
  .hc/hc.db                   submission history (SQLite)
  .hc/venv/                   Python virtual environment managed by hc
  pyrightconfig.json          points pyright at .hc/venv
```

`hc init --lang python` makes Python the default solution language (`language` in `hc.yaml`; default `go`) and creates the Python environment described below right away. If that fails, for example because neither uv nor Python 3.10+ is installed, `hc init` warns and creates the workspace anyway.

`hc` finds the workspace from `-w <dir>`, then `$HC_WORKSPACE`, then the nearest parent directory containing `hc.yaml`, then `~/handcoding`. You can run it from anywhere.

Each Go problem is a standalone module, and the driver sits next to `solution.go`. So gopls, `go build` and Delve breakpoints all work on your real file. The generated files are rewritten before every run, so don't edit them. `go.mod` follows your Go: its `go` line is your toolchain's version (`go 1.27` for Go 1.27.2), so your solutions can use everything it supports. hc builds with `GOTOOLCHAIN=local` and never downloads another toolchain.

Python solutions run in a virtual environment that hc manages in `.hc/venv`. hc creates it the first time a command needs Python (`hc init --lang python`, `hc start`, `test`, `submit`, `debug`, `cases build`, `validate --run`):

1. with [uv](https://docs.astral.sh/uv/) if it is installed: `uv venv --python '>=3.10'`, which downloads a Python if none is installed;
2. otherwise with the newest Python 3.10+ on PATH (`python3`, `python` or `python3.N`) and `-m venv`. On Debian and Ubuntu this needs the `python3-venv` package.

The environment holds debugpy for headless debugging. Downloading Python and installing debugpy need network access, once; judging stays offline. If the environment stops working (the workspace was moved, or its Python removed), hc creates it again. `hc env sync` recreates it on demand, for example after installing uv. hc also writes `pyrightconfig.json` at the workspace root, unless you have one, so pyright and basedpyright check your solutions with the same interpreter. To use your own Python instead (conda, a custom build), set `python.interpreter` in `hc.yaml`; hc then creates no environment.

Consider running `git init` in the workspace to keep a record of your practice.

## The interactive list

Run `hc` (or `hc list`) to open the problem list. It works best with the terminal split in two: `hc` on one side, your editor on the other.

- Press `enter` on a problem to open it. The statement is shown, the solution file is created from the template, and its absolute path is **copied to the clipboard**. In the editor, `:e <paste>` and start writing.
- Test, submit, debug and history are all available from the statement. Results appear in the TUI, with your program's own output listed separately below them.
- The debugger takes over the terminal. The screen is cleared before it starts, and the TUI comes back when it exits. If the last run had a failing case, `d` debugs that case.
- Statement images are drawn in the terminal. kitty and Ghostty show the original image. Other truecolor terminals show a block-character approximation that loses detail; press `o` to open the original in your image viewer. See [`images`](#configuration) to choose the mode.
- Statement colors follow the terminal background. If they look wrong, set `theme` in `hc.yaml`.

| Key | Action | Key | Action |
|---|---|---|---|
| `enter` | Open the problem (and copy the solution path) | `e` | Open the solution in your editor |
| `t` | Test on the examples | `s` | Submit (all cases) |
| `d` | Debug (the last failing case, if any) | `h` | Submission history: code, judge details, restore, delete |
| `y` | Copy the solution path again | `L` | Switch language |
| `r` | Reset to the template | `D` | Discard: delete the solution and its history, back to not started |
| `/` | Search | `f` | Filter: all / unsolved / solved |
| `o` | Open the statement's images (statement view) | | |
| `esc` / `q` | Back; `q` on the list quits | `ctrl+c` | Quit |

## Commands

```
Practice       list  start  edit  show  test  submit  debug  reset  discard  history
Problem bank   init  fetch  new  import  cases  validate
Coding agent   gen  fix  skill
```

Problems can be referred to by slug, slug prefix, number or directory name: `hc test 1`, `hc test two`.

```sh
hc start two-sum -e                                 # create the solution and open the editor
hc test two-sum                                     # run the examples
hc test two-sum -c 4 -c 5                           # run chosen cases
hc test two-sum --all -v                            # all cases, full output, not recorded
hc test two-sum -i '{"nums":[1,2,3],"target":5}'    # custom input (expected output from the reference, if any)
hc test lru-cache -l py                             # solve in Python
hc submit two-sum                                   # all cases, recorded in history

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
hc fetch --site cn 1        # from leetcode.cn (Chinese statement by default)
hc fetch https://leetcode.cn/problems/two-sum/description/   # any problem URL; the URL picks the site
hc fetch --site cn "LCR 001"                                  # leetcode.cn IDs such as LCR/LCP
hc gen 146                  # make it ready: test cases and the judge rule
```

`hc fetch` downloads the statement and its images, the signature, the Go and Python starter code, and the examples. Expected outputs are included when they can be parsed from the statement.

A fetched problem is **not ready to submit** yet. LeetCode only publishes the examples, and not how answers are judged: whether order matters, whether several answers are valid. Until that is decided, `hc test` shows your output next to the example's answer without a verdict, and `hc submit` refuses. `hc gen` writes the full test suite and decides the judge rule from the statement, writing a checker when no built-in rule fits; you can also set `judge.compare` yourself (see the [problem protocol](protocol.md#problemyaml)). The list marks such problems as "not ready".

Premium problems are not available. Neither are problems whose signature uses types hc does not support yet (such as `MountainArray` in interactive problems); `hc fetch` says so.

### Your own problems

```sh
hc new my-problem --kind function   # scaffold a problem (--kind class for a design problem)
$EDITOR ~/handcoding/problems/my-problem/problem.yaml
hc validate my-problem              # check the signature and the cases as you go
hc gen my-problem                   # have an agent write a reference and cases, or write them yourself:
hc cases build my-problem           #   expected outputs from reference/solution.*
hc validate my-problem --run        #   every reference must pass every case
hc import ./my-problems/            # import problem directories shared by others
```

The [problem protocol](protocol.md#writing-a-problem-by-hand) walks through a complete function problem and a design problem, and shows how to write the reference solution. `hc import` copies only the files the protocol defines, and refuses problems whose slug or ID is already in the bank (`--force` replaces a problem with the same slug).

### Test cases

```sh
hc cases list 3                     # show the cases
hc cases add 3 -i '{"s":"abba"}'    # append a case (expected output from the reference, if any)
hc cases edit 3                     # edit cases.jsonl in your editor
hc cases build 3                    # fill in missing expected outputs from the reference (--force: all)
```

## Test cases from a coding agent

```sh
hc gen 3                                                   # Claude Code by default
hc gen 3 --agent codex -n 40 -m "more strings of one repeated character"
hc fix two-sum -c 5 -m "answers in a different order should pass"
```

`hc` runs the agent headless in the workspace (`claude -p` or `codex exec`). Before it starts, it installs or updates the skill the agent follows, **`hc-cases`** (see [Agent skills](#agent-skills)). Following the skill, the agent:

1. writes a reference solution, `reference/solution.py`, and a brute-force one to cross-check when that helps;
2. writes test **inputs** to `cases.jsonl`, covering edge cases, special structures and large inputs;
3. runs `hc cases build`, so expected outputs come from running the reference, not from the model;
4. runs `hc validate --run` until every reference passes.

To use another agent, set its command line in `hc.yaml`; `{prompt}` is replaced by the prompt:

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

### Agent skills

hc ships skills that teach coding agents its problem protocol and workflows. `hc init` installs the ones a workspace gets (today `hc-cases`: judge rules, reference solutions, checkers and test cases) into `.claude/skills/` for Claude Code and `.agents/skills/` for Codex, and lists them in a block of `AGENTS.md`. Because they live in the workspace, any Claude Code or Codex session started there can use them: ask it to "generate cases for this hc problem".

```sh
hc skill list               # each skill: installed, up to date, outdated or modified, and by which hc version
hc skill install            # install or update the workspace's skills (after upgrading hc)
hc skill install --force    # also replace the skills you edited
hc skill show hc-cases      # print a skill
```

hc marks every copy it installs, with its version and a hash of the content (`.hc-skill.json` in the skill's directory), and replaces or removes only those: a skill you edited is kept unless you pass `--force`, and skills hc did not install are never touched. In `AGENTS.md`, hc rewrites only the block between `<!-- hc:begin -->` and `<!-- hc:end -->`; the rest of the file is yours. `hc gen` and `hc fix` install missing or outdated skills before they start the agent, and warn about edited ones instead of replacing them; `hc skill install` without names also removes the skills hc installed that the workspace no longer gets.

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
```

`images: auto` picks `kitty` in kitty, Ghostty and [herdr](https://herdr.dev), `blocks` in other truecolor terminals, and `none` otherwise.

- **herdr** draws kitty graphics itself, so images show whenever the terminal around herdr supports kitty graphics: kitty, Ghostty, or WezTerm with `enable_kitty_graphics = true`. In other terminals the image area stays blank; set `images: blocks`, and press `o` to see the original.
- **tmux**: auto never picks kitty. If the outer terminal supports Unicode placeholders (kitty, Ghostty), set `images: kitty` and enable tmux's `allow-passthrough`.
- **WezTerm** supports kitty graphics but not Unicode placeholders, so `images: kitty` shows garbled characters there. Keep `blocks` when running WezTerm directly.

## Current limitations

- Interactive problems aren't supported yet.
- `hc fetch` imports problems with [structure inputs](protocol.md#structure-inputs) (a list with a cycle, intersecting lists, node references such as Lowest Common Ancestor), working out the structures from the difference between LeetCode's test data and its template. Example outputs that are not plain JSON, such as 142's "tail connects to node index 1", are left to `hc gen`. The samples 141, 142, 160, 236, 237 and 1379 show these problems.
- `hc fetch` imports problems with [LeetCode's `Node` types](protocol.md#leetcodes-node-types), `NestedInteger` and `Employee`, reading their signature from LeetCode's Go template. `returns_copy` (Clone Graph) is left to `hc gen`, and so are the examples of iterator problems such as Flatten Nested List Iterator, which LeetCode gives without the calls.
- Problems that encode a value and decode it back (297, 449, 535) are judged as [round trips](protocol.md#round-trip-problems): only the decoded value is compared, so requirements on the encoding itself (449: "as compact as possible") are not checked. `hc fetch` imports them, reading the encoder, the decoder and whether they share an instance from the harness in LeetCode's template comment. The samples 297 and 535 show both kinds.
- Run time is measured, but memory is not limited.
- LeetCode premium problems cannot be fetched.
- Problems with many valid answers that cannot all be listed ("return any valid answer") and randomized problems are judged by a [checker](protocol.md#checkers), which `hc gen` writes; the samples 5, 108, 210, 324, 384 and 528 have one. Checkers are Python, so these problems need Python 3.10 or later even when you solve them in Go; the virtual environment hc manages provides it.
