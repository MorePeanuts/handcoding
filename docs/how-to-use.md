# Using handcoding

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
```

`hc` finds the workspace from `-w <dir>`, then `$HC_WORKSPACE`, then the nearest parent directory containing `hc.yaml`, then `~/handcoding`. You can run it from anywhere.

Each Go problem is a standalone module, and the driver sits next to `solution.go`. So gopls, `go build` and Delve breakpoints all work on your real file. The generated files are rewritten before every run, so don't edit them.

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
```

`hc fetch` downloads the statement and its images, the signature, the Go and Python starter code, and the examples. Expected outputs are included when they can be parsed from the statement. LeetCode only publishes the examples, so the full test suite comes from `hc gen`. Premium problems are not available.

### Your own problems

```sh
hc new my-problem --kind function   # scaffold a problem
$EDITOR ~/handcoding/problems/my-problem/problem.yaml
hc validate my-problem
hc gen my-problem                   # have an agent write a reference and cases
hc import ./my-problems/            # import problem directories shared by others
```

The format is described in the [problem protocol](protocol.md).

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

`hc` runs the agent headless in the workspace (`claude -p` or `codex exec`). It also installs the **handcoding skill** there: `.claude/skills/handcoding/SKILL.md`, and `AGENTS.md` for Codex. Following the skill, the agent:

1. writes a reference solution, `reference/solution.py`, and a brute-force one to cross-check when that helps;
2. writes test **inputs** to `cases.jsonl`, covering edge cases, special structures and large inputs;
3. runs `hc cases build`, so expected outputs come from running the reference, not from the model;
4. runs `hc validate --run` until it passes.

After `hc skill install --global`, you can also ask any Claude Code session to "generate cases for this hc problem".

To use another agent, set its command line in `hc.yaml`; `{prompt}` is replaced by the prompt:

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

## Debugging

```sh
hc debug two-sum -c 2       # Go: Delve, stopped at your function
hc debug lru -l py          # Python: pdb
```

### Neovim (nvim-dap)

```sh
hc debug two-sum -c 2 --headless    # Go: dlv headless on 127.0.0.1:38697
hc debug lru -l py --headless       # Python: debugpy (pip install debugpy)
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
timeout: 10s          # wall-clock limit per run
python:
  interpreter: python3
agent:
  name: claude        # claude | codex, or set command (see above)
leetcode:
  site: com           # com | cn
```

`images: auto` picks `kitty` in kitty and Ghostty, `blocks` in other truecolor terminals, and `none` otherwise. Inside tmux, auto never picks kitty. If the outer terminal supports it, set `images: kitty` and enable tmux's `allow-passthrough`.

## Current limitations

- Some signatures are not supported yet: `Node` (graphs, N-ary trees, lists with random pointers), `NestedInteger`, linked lists with cycles given as input. Problems that need a special judge (checker) aren't supported either.
- Run time is measured, but memory is not limited.
- LeetCode premium problems cannot be fetched.
