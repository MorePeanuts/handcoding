# Writing packs

English · [简体中文](packs_zh.md)

A [pack](protocol.md#packs) is a set of problems published together, usually a git repository of its own, that anyone can install into their workspace with `hc pack install`. This guide is about writing one: creating the repository, adding problems, checking them in CI, and publishing releases. The format of a pack and of its problems is the [problem protocol](protocol.md); this guide links to it rather than repeating it.

- [Creating a pack](#creating-a-pack)
- [Adding problems](#adding-problems)
- [CI and the pinned hc version](#ci-and-the-pinned-hc-version)
- [Publishing](#publishing)
- [Trust](#trust)

## Creating a pack

```sh
hc pack init my-pack --title "My pack"   # or, in an empty directory: hc pack init
cd my-pack
git init                                  # unless it already is a repository
```

`hc pack init [dir]` scaffolds a pack in `dir` (the current directory by default), and creates the directory if it is missing:

```
pack.yaml                     the manifest: protocol, name, title and an empty groups list
problems/                     the problems, one directory each (empty, with a .gitkeep)
.gitignore                    what hc adds to a checkout: .hc/, solutions/, hc.yaml, pyrightconfig.json
.github/workflows/check.yml   CI: hc pack check on every push and pull request
AGENTS.md                     instructions for coding agents, and hc's skill index
CLAUDE.md                     @AGENTS.md, and room for notes for Claude Code
.claude/skills/               the skills for writing a pack, for Claude Code
.agents/skills/               the same skills, for Codex
```

Commit all of it. The skills are committed on purpose: a fresh clone describes itself to whichever agent opens it, and CI checks that they belong to the hc it pins (see [below](#ci-and-the-pinned-hc-version)). Write your own notes for agents in `AGENTS.md`, outside the block between `<!-- hc:begin -->` and `<!-- hc:end -->`, which hc rewrites; `CLAUDE.md` is yours.

- **The name** is `--name`, or else the directory's name in kebab-case (`My Pack` becomes `my-pack`). It is the pack's identity: installed packs, solutions and history are keyed by it, so choose it before you publish. Its rules are those of [`pack.yaml`](protocol.md#packyaml).
- **Refusals.** `hc pack init` refuses a directory that already holds `pack.yaml` or is inside a workspace, and it never overwrites a file: when any file it would write exists, it lists them and writes nothing.
- **git.** hc does not run `git init`. If `dir` is in a git repository already it stays one, but GitHub runs only the workflows at the root of a repository, so the pack should be the repository's root.

The directory is now a pack checkout, which hc opens as a workspace as it is (see [working on a pack](how-to-use.md#working-on-a-pack)): every hc command run in it works on this pack.

`pack.yaml` holds more than `hc pack init` writes: a `version`, a `description`, `authors`, a `license`, a `homepage`, the `languages` the problems can be solved in, and `groups`, which arrange the problems into ordered lists such as topics or study plans. `hc docs protocol packyaml` prints the fields, and `hc docs protocol groups` the rules of groups.

## Adding problems

```sh
hc fetch two-sum            # import a problem from LeetCode
hc new my-problem           # or write one of your own (problems/my-problem/)
hc import ../elsewhere/     # or copy problem directories in
hc gen two-sum              # an agent writes a reference, the cases and the judge rule
hc validate two-sum --run   # every reference passes every case
hc pack check               # the whole pack, as CI checks it
```

The flow is that of a practice workspace ([your own problems](how-to-use.md#your-own-problems), [test cases from a coding agent](how-to-use.md#test-cases-from-a-coding-agent)), except that problems go to the pack's `problems/` and are named by their slug alone. Then add the slug to a group of `pack.yaml` if the pack has groups.

A problem lands only when it passes `hc pack check`: it is valid, [ready](protocol.md#readiness) (its judge rule, `judge.compare`, is decided), every case has an expected output, and every reference solution passes every case. References must work on the oldest toolchains the protocol supports ([language versions](protocol.md#language-versions)), and CI runs the Python ones on Python 3.10. `hc fetch` deliberately leaves the judge rule undecided, since deciding it means reading the statement: that is part of `hc gen`.

If a problem cannot be expressed in the protocol, do not work around it (with a checker that judges something else, or a statement changed to fit): open an [issue](https://github.com/MorePeanuts/handcoding/issues) that says what is missing, and leave the problem out until the protocol has it. The `AGENTS.md` that `hc pack init` writes tells agents the same.

## CI and the pinned hc version

`.github/workflows/check.yml` runs on every push and pull request. It sets up Go and Python 3.10, the oldest Python the protocol supports, so that references and checkers are checked where users may run them, and then:

1. installs hc with `go install github.com/MorePeanuts/handcoding/cmd/hc@<version>`;
2. writes a `hc.yaml` that sets `python.interpreter: python3`, so hc judges with that Python rather than creating a virtual environment (`hc.yaml` is not committed);
3. runs `hc pack check`;
4. runs `hc skill install --force` and fails if that changes any committed skill, or `AGENTS.md`: the skills in the repository must be those of the hc that checks the pack.

**The pin.** `<version>` is the version of the hc that ran `hc pack init`. A development build of hc has no release to pin, so the workflow then installs `@latest`, with a comment saying to pin it: replace `latest` with a release such as `v0.2.0`. A pinned hc keeps CI from changing under you when hc releases.

**When to bump it.** The pinned hc decides which [protocol versions](protocol.md#protocol-versions) CI reads and how strictly it checks them. Bump it when you want what a newer hc brings: a feature of a newer protocol, or a fix to its checks. Before you bump, read what changed, with `hc docs protocol protocol-versions` from the new hc, and the release notes. Bumping hc does not change the `protocol` of `pack.yaml`. Raise that only to use a feature it introduced, and know that hc versions that do not read the new protocol then refuse the pack.

**Refreshing the skills.** The skills belong to an hc version. After a bump, install the same hc locally, then:

```sh
hc skill install     # replace the skills with those of this hc
git diff             # review what changed, then commit it with the bump
```

Without this, the last step of CI fails. CI runs `hc skill install --force`, so the committed copies of hc's skills must be exactly the pinned hc's: an edit to one of them fails CI too (`hc skill list` shows it as *modified*). Keep your own guidance in `CLAUDE.md`, the author section of `AGENTS.md`, or a skill of your own, which hc never touches.

## Publishing

Push the repository to a git host. Users install it with its URL:

```sh
hc pack install https://github.com/you/my-pack.git               # the default branch
hc pack install https://github.com/you/my-pack.git --ref v0.3.0  # a release
```

- **Releases** are git tags. Tag a commit whose CI passed, as `v0.3.0`, and set `version: 0.3.0` in `pack.yaml` in that commit (SemVer, without the `v`; see [`pack.yaml`](protocol.md#packyaml)). The `version` is informational: hc pins installs by commit, and shows the version in `hc pack list`.
- **Updates.** `hc pack update` moves an install that follows a branch to its latest commit, and leaves one that follows a tag where it is, unless the tag moved: users switch releases with `hc pack update <name> --ref v0.4.0`. So do not move a tag once it is published.
- **Breaking changes.** Users' solutions and history are keyed by the pack's name and by each problem's slug and directory ([identity](protocol.md#identity)). Renaming the pack makes it another pack, and hc refuses to update an install across a rename. Renaming or removing a problem leaves users' work for it orphaned: hc keeps it and says so, but no problem uses it any more. Keep slugs and directory names stable once published.

## Trust

A pack is code. Installing a pack means running its checkers and reference solutions on your machine: checkers whenever a problem judged by one is tested or submitted, references under `hc cases build` and `hc validate --run`, and both under `hc pack check`. hc does not sandbox them. So:

- as an author, keep references and checkers to computing answers: no network, no files outside their inputs, nothing a reader would not expect from a solution;
- as a user, install packs only from sources you trust, and review a pack (its `reference/` and `checker.py` files) before you install or update it;
- in CI, the workflow runs with read-only access to the repository (`permissions: contents: read`), so a pull request that changes a reference cannot push to it.
