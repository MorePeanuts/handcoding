<div align="center">

# handcoding

**Practice coding problems by hand, in your own editor, judged on your own machine.**

[![CI](https://github.com/MorePeanuts/handcoding/actions/workflows/ci.yml/badge.svg)](https://github.com/MorePeanuts/handcoding/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/MorePeanuts/handcoding?include_prereleases&sort=semver)](https://github.com/MorePeanuts/handcoding/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/MorePeanuts/handcoding)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

English · [简体中文](README_zh.md)

</div>

<p align="center"><img src="docs/images/demo.gif" alt="Solving Two Sum with hc: a failing test, a fix in Neovim, then submit and review the history"></p>

> [!WARNING]
> handcoding is in early development (`v0.x`). Until 1.0, the CLI, the configuration, the problem format and the data layout may change incompatibly, and an upgrade may require re-initializing your workspace.

## Why

Coding agents now write a large share of the code that ships, and for experienced developers that is a real gain. But reviewing, steering and debugging that code depends on judgment, and that judgment comes from having written a lot of code yourself. You still have to practice: read a problem, pick a data structure, get an index off by one, and find the bug in a debugger.

Online judges make that practice harder than it needs to be:

- You need to be online.
- The debugger is a paid feature.
- Hidden test cases tell you that you failed, not why.
- You code in a browser instead of the editor you know.

Command-line clients bring your editor back, but they still send your code to the cloud to be judged.

**handcoding (`hc`) runs the whole loop locally.** Problems live on disk. You write solutions in Neovim, VS Code or whatever you like. `hc` builds, runs, judges and debugs them on your machine, and every test case is a file you can read.

## Features

- **Local judge.** Supports Go and Python, plus design problems (`LRUCache`), in-place problems, linked lists, binary trees, character grids, floating-point tolerance, any-order answers, and checkers for problems with many valid answers or random output, and encoders judged by decoding (Serialize and Deserialize Binary Tree). No network needed.
- **Real debuggers.** `hc debug` starts Delve or pdb stopped at your function. `--headless` serves DAP for nvim-dap or VS Code.
- **Interactive TUI.** A problem list in the style of rustlings, with the statement, its images, test results and submission history in one place. Your editor stays in the next pane.
- **LeetCode import.** `hc fetch 146` downloads the statement, its images, the signature, starter code and the examples.
- **Agent-written test cases.** `hc gen` has Claude Code or Codex decide how a problem is judged (any order? several valid answers?) and write a reference solution and test inputs. Expected outputs come from running the reference, not from the model's guess. `hc fix` repairs a wrong case.
- **Open problem format.** Problems are plain directories: YAML, Markdown and JSON Lines. Write your own and share them as packs: git repositories that `hc pack install` installs and `hc pack update` keeps current.
- **History.** Every submission keeps its code and judge report, so you can review, restore or delete it.

## Install

```sh
go install github.com/MorePeanuts/handcoding/cmd/hc@latest
```

Prebuilt binaries for Linux, macOS and Windows are on the [releases page](https://github.com/MorePeanuts/handcoding/releases).

To solve problems you need the toolchain of your language: Go 1.22+, or for Python, [uv](https://docs.astral.sh/uv/) (recommended) or Python 3.10+. hc runs Python solutions in a virtual environment of its own, created in the workspace the first time it needs Python. With uv, that works even where the system Python is too old (macOS ships 3.9): uv downloads a newer one. Optional extras:

- [Delve](https://github.com/go-delve/delve) for debugging Go;
- `claude` or `codex` for generating test cases.

## Quick start

```sh
hc init                 # create an empty workspace in ~/handcoding
hc fetch two-sum        # import a problem from LeetCode
hc gen two-sum          # let a coding agent write its test cases and judge rule
hc                      # open the problem list
```

To solve in Python by default, run `hc init --lang python` instead; it also sets up the Python environment right away. Your problems live in the workspace's `packs/local/problems/`: `hc new` scaffolds one of your own, and `hc import` copies problem directories in. A new workspace starts with no problems: fetch them one by one, or install a pack of ready-made problems with `hc pack install <git-url>`, which clones it into `packs/` beside your own and records it in `hc.yaml` (see [packs](docs/how-to-use.md#packs)).

Press `enter` on a problem. `hc` shows the statement, creates `solution.go` from the template and copies its path to the clipboard. Open it in your editor next door, then press `t` to test and `s` to submit.

> [!TIP]
> Statement images are shown at full resolution in kitty and Ghostty, and in [herdr](https://herdr.dev) when the terminal around it supports kitty graphics. If images inside herdr come out blank, your terminal lacks kitty graphics: set `images: blocks` in `hc.yaml`. See [statement images](docs/how-to-use.md#configuration).

Every action also has a command:

```sh
hc start two-sum -e     # create the solution and open it in your editor
hc test two-sum         # run the examples
hc submit two-sum       # judge on all cases and record the submission
hc debug two-sum -c 4   # debug case 4
```

## Documentation

- [Usage guide](docs/how-to-use.md): the TUI, every command, configuration, debugging and agents
- [Problem protocol](docs/protocol.md): the specification of the problem format, for writing your own problems; `hc docs protocol` prints it, or one section of it
- [Contributing](CONTRIBUTING.md): development, and how to add a language

## Roadmap

Planned:

- more languages: C, C++, Rust, Java, JavaScript, TypeScript;
- problem packs with an official LeetCode bank;
- notes, and AI hints that never give away the answer;
- an open problem store.

Progress is tracked in the [milestones](https://github.com/MorePeanuts/handcoding/milestones).

## Contributing

Bug reports and ideas are welcome in [issues](https://github.com/MorePeanuts/handcoding/issues). See [CONTRIBUTING.md](CONTRIBUTING.md) for how to build and test.

## License

[MIT](LICENSE)
