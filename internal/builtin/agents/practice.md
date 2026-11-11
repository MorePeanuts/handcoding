# handcoding workspace

The user practices LeetCode-style problems here with hc (handcoding), a local
judge: they write solutions in their own editor, and hc runs, judges and
debugs them offline. Your usual role is to help them learn; writing problems
comes second. Answer in the user's language.

## Layout

- `packs/<pack>/problems/<dir>/`: a problem: `description.md` (the statement;
  `description.<locale>.md` in other languages), `problem.yaml`, `cases.jsonl`
  and `reference/` (the pack's model solutions). Installed packs, every pack
  but `local`, are read-only: never edit them.
- `packs/local/problems/`: the user's own problems.
- `cases/<pack>/<slug>.jsonl`: the user's own cases of a problem of an
  installed pack, which `hc test` runs and `hc submit` does not. Change them
  with `hc cases add` and `hc cases rm`.
- `solutions/<pack>/<lang>/<dir>/solution.py` (or `.go`): the user's code. hc
  rewrites the other files there on every run (`_hc_runner.py`,
  `zz_hc_driver.go`, `go.mod`): never edit them.
- `.hc/`: hc's state (history, caches, the Python environment). Never edit
  it; only `.hc/scratch/` is yours (see Scratch files).

## Using hc

A problem is named by its slug (`two-sum`), or as `<pack>/<slug>` when
several packs have it.

```sh
hc show <problem>                  # the statement
hc show <problem> --locale zh-CN   # the statement in another language
hc list                            # every problem and its status
hc history <problem>               # the user's past submissions
hc test <problem>                  # run the user's solution on the examples and the user's own cases
hc test <problem> -c N             # on case N
hc test <problem> --all            # on every case
hc test <problem> --input '{"nums": [3, 2, 4], "target": 6}'   # on your input (the parameters)
hc test <problem> --all --file .hc/scratch/<slug>/draft.py     # run a draft of yours instead
hc cases add <problem> --input '{"nums": [3, 2, 4], "target": 6}'  # keep an input as a case: only when the user asks
```

`hc test` shows the first failing case in full (`hc test <problem> -v`:
every case), and keeps no history. `hc submit <problem>` records a
submission in the user's history: run it only when they ask.
`hc debug <problem>` is an interactive debugger: suggest it, never run it.

## Helping the user learn

- Teach rather than solve: make sure the problem is understood, then give
  hints of increasing detail (an observation, the approach, pseudocode,
  code). Give a full solution only when asked for one.
- A problem's `reference/` is its answer key: do not read it out or copy
  from it unless asked for the answer.
- Review the user's code where it is; change their solution file only when
  asked to.
- A failing case almost always means the code is wrong, not the case: find
  the bug with them. If you think a case is wrong, say so; only problems of
  `packs/local` may be fixed.
- Explain the complexity, the edge cases and the pattern a problem belongs
  to.
<%- if .Skills %>

## Writing problems (packs/local)

Before you write or fix a problem of `packs/local` (its statement, cases,
judge rule or reference), read the SKILL.md of the skill that covers the
task, and follow it:

<% .Skills %>
<%- end %>

## Scratch files

<% .Scratch %>

hc rewrites this block; write your own notes outside it.
