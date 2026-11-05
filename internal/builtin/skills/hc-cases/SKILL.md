---
name: hc-cases
description: Decide how an hc (handcoding) problem is judged and write its test cases. Use when generating or fixing the test cases (cases.jsonl) of a problem in an hc workspace, choosing its judge rule (judge.compare in problem.yaml), writing or fixing a reference solution (reference/solution.py or .go), writing a checker (checker.py), or generators of large inputs (generator.py); `hc gen` and `hc fix` use it. For the problem's definition itself (signature, structure inputs, statement, templates), use hc-problem instead.
---

# handcoding test case authoring

`hc` is a local judge for LeetCode-style problems. A workspace contains
`hc.yaml`; its own problems live in `packs/local/problems/<dir>/`. Other
directories of `packs/` are installed packs: read-only, so never edit them
(hc refuses to). A pack checkout (a directory with `pack.yaml`) is a
workspace too: the pack is its only pack, and its problems in
`problems/<dir>/` are yours to edit. The user's own code is in
`solutions/<pack>/<lang>/<dir>/`. **Never read or modify the user's solutions unless asked to diagnose a
reported failure, and never change them.**

In hc commands, name a problem by its slug, or as `<pack>/<slug>` when the
slug is in several packs (`local/two-sum`); `hc gen` and `hc fix` tell you
the name to use.

This skill covers how a problem is judged: its judge rule, its reference
solutions, its checker and its test cases, generators included. What the problem is (directory,
signature, structure inputs, statement, templates) is the `hc-problem`
skill. If you find that the definition is wrong (a parameter the template
does not have, a node that should be a `ref`, a missing `returns_copy`, a
wrong `Node` type), fix it with `hc-problem` first, then come back.

## The spec

The facts (the case format, compare modes, the checker interface, what a
reference may import) are in the problem protocol that hc embeds. This
skill does not repeat them: when a step names a section, read it with
`hc docs protocol <section>` before you write that part, and follow it
exactly. `hc docs protocol --list` lists the sections. When this skill and
the spec seem to disagree, the spec wins.

## Decide the judge rule

A problem without `judge.compare` is not ready (`hc docs protocol readiness`),
and deciding it is your job. Write it to problem.yaml even when it is
`exact`. Read `hc docs protocol compare-modes`, then decide from the
statement, not from the examples:

- exactly one correct output: `exact`;
- "return the answer in any order", "the order does not matter":
  `unordered` when only the outer list is free, `unordered_deep` when the
  inner lists are free too (subsets, groups: yes; permutations: no, each
  permutation's own order matters);
- several valid answers that can all be listed, a handful per case:
  `any_of`, except in a class problem that is not a round trip, whose
  `expected` holds one entry per op: use `checker` there;
- many valid answers ("return any of them", "any valid order", any
  height-balanced BST), or a randomized output: `checker`, and write
  `checker.py` ([Checkers](#checkers)). Prefer a built-in mode when one
  fits;
- a round trip (`judge.round_trip`): `exact`, since the decoded value must
  equal the input. A requirement on the encoded string (449's "as compact
  as possible") is not judged; say so in your summary instead of inventing
  a check.

Then the options:

- `double` results: the default tolerance usually fits. When the statement
  names one ("answers within 10^-5 of the actual answer are accepted"), set
  `judge.tolerance` to it; `0` compares exactly.
- In-place problems: `judge.output` and `output_length` belong to the
  definition and combine with every mode, `checker` included. Keep them
  (`hc docs protocol in-place-output`).
- A run has a wall-clock limit that guards against hangs, not slow
  solutions (`hc docs protocol run-limit`). It is the user's setting, not
  the problem's: if the reference needs a large part of the default (10s)
  on your largest case, make the case smaller.

## Reference solutions

Read `hc docs protocol solution-abi`, and `hc docs protocol python` or
`hc docs protocol go` for the language you write. Expected outputs come
from running the reference, never from you, so the reference must be
right.

**Write a reference in every language the pack is solved in.** A solution
is judged only in a language the problem has a reference in
(`hc docs protocol readiness`). The languages are `languages` in
`pack.yaml`, every language when it has none, and every language for the
workspace's own problems; `hc validate <slug>` names a missing reference.

- Write `reference/solution.py` first. It must run on Python 3.10, the
  oldest version hc supports (`hc docs protocol language-versions`); import
  only the public names of `hc_runtime` and the standard library.
- Write an idiomatic implementation of the complexity the problem accepts,
  the way a good solution would: solutions are timed against it
  ([Time limits](#time-limits)), so neither a slow-constant version nor a
  hyper-optimized one. A slow constant raises the limit of every solution
  and can let a worse complexity pass: of the usual solutions, take the
  one with the least work per step (two pointers rather than a hash set
  for each element). If it is not obviously correct, also write a brute
  force and cross-check the two on small random inputs with a throwaway
  script before you trust it.
- A Go reference starts with `package main` (LeetCode's Go template omits
  it) and must build with Go 1.22: no newer language features or standard
  library functions (`slices.Sorted`, `maps.Keys`, range over a function).
  `hc validate --run` checks this and names what is too new.

**Prove the rule with the second reference.** When more than one output is
acceptable, write `reference/solution.go` so that it returns a
*different* valid answer than the Python one wherever the rule allows:
another order, another valid answer, another (correct) random algorithm.
`hc validate --run` runs both against the same expected outputs, so a rule
that is too strict fails there. Make it differ in what it returns, not in
its cost: it times the Go solutions, so it follows the bullet on the
complexity and constant above.

**Round trips:** give the Go reference another encoding than the Python
one (preorder instead of level order, a hash instead of a counter), so the
cases do not depend on one format.

## Checkers

Read `hc docs protocol checkers` for the interface: the arguments, what to
return, the time limit, what may be imported. Then:

- Check validity from `input`, and optimality against `expected`, the
  reference's answer: one valid answer among many. Use it for the optimal
  value instead of solving the problem again (the longest palindrome's
  length, the size of the largest subset, the minimum sum).
- Validate `output`'s type and shape before you use it, so that a wrong
  output is a rejection, not a crash: a checker that raises fails the
  whole run as a broken problem.
- Return a message that says what is wrong (`"nums[3] = 2 is not less than
  nums[4] = 2"`); users see it with the Wrong Answer.

**Prove the checker rejects wrong answers.** Copy `reference/solution.go`
aside, replace it with a plausible wrong solution (an invalid answer, a
valid but not optimal one, always the same random pick), and run
`hc validate <slug> --run`: it must report Wrong Answer, with a helpful
message, on the cases you expect. Try each kind of wrong solution, then
restore the real reference and run it again.

### Randomized problems

Shuffle an Array, Random Pick with Weight, Linked List Random Node and
`getRandom` are class problems whose cases make many calls, hundreds to a
few thousand. The spec requires only that a correct solution is
essentially never rejected; this is how to make sure:

1. Check every output's validity, as for any checker.
2. Compare deterministic calls (`reset`) with `expected`. The random calls'
   `expected` is just one sample of the reference: never compare them.
3. Test the distribution: count outcomes per bucket (value, position,
   index) and accept each count within a bound derived from a tail bound,
   not from a guess. With n random calls in the case, a bucket of
   probability p and k buckets tested, Hoeffding's inequality and a union
   bound give a false-reject probability of at most δ when every count
   lies within `n*p ± sqrt(n * ln(2*k/δ) / 2)`. Use δ = 1e-6 or less.
4. Write the false-reject probability per case in a comment of the
   checker, with how you derived it.
5. Make sure the bound still rejects an obviously non-random solution
   (always the same pick, the identity shuffle): prove it with the wrong
   solutions above. If it does not, the case needs more calls.

## Test cases

Read `hc docs protocol cases` for the format, and the section of every
type and structure the problem uses (`hc docs protocol types`,
`hc docs protocol structure-inputs`, `hc docs protocol round-trip-problems`):
case inputs must satisfy their rules as well as the statement's.

- Keep existing lines; never drop `"example": true` cases.
- Write inputs **without** `expected`: `hc cases build` fills it from the
  reference, `checker` mode included. With `any_of`, write `expected`
  yourself as the array of every acceptable answer (compute them with a
  throwaway script); `hc cases build` refuses, since a reference finds only
  one. That includes the examples.
- Aim for 15 to 40 cases. What they cover matters, not how many; cover:
  - every statement example (`"example": true`);
  - minimum sizes and empty inputs the constraints allow;
  - boundary values (minimum and maximum values, negative numbers,
    duplicates);
  - tricky structures (skewed trees, one node, all equal, sorted and
    reverse sorted; for structure inputs, no cycle, no intersection, a
    node at the root or the tail);
  - large inputs near the constraint limits, as
    [generated cases](#generated-cases); when the problem tests
    complexity, most of them in the worst shape of a slower solution, so
    that those take most of the reference's time
    ([Time limits](#time-limits)).
- Every input must satisfy the stated constraints and guarantees ("exactly
  one solution", "all values are unique"), by construction: for "exactly
  one solution", build the answer in and keep every other pair from
  forming one. Never retry random inputs until one fits: for large inputs
  such a loop may never end, and any retry loop in a script needs a bound.
- Large inputs are generated cases, never stored ones: a stored case over
  100 KB fails `hc pack check`.
- Give each non-example case a `note` saying what it targets.
- Randomized problems: a few cases with many calls (1000 to 3000) are
  better than many small ones; the checker needs samples.

**Prove the cases reject wrong solutions.** Cases that a plausible wrong
solution passes are not enough, however many there are. Write two or three
wrong solutions a user might write for this problem: an off-by-one, one
that mishandles an input the constraints allow (empty, one element,
duplicates, negative numbers), a greedy or shortcut that the problem
defeats. For each, copy a reference aside, put the wrong solution in its
place, and run `hc validate <slug> --run`: some case must fail. If none
does, add a case that catches it, with a `note` naming the mistake. Restore
the real reference before anything else, and never run `hc cases build`
while a wrong solution is in place: it would write wrong expected outputs.
A slow but correct solution is a matter of [time limits](#time-limits).

## Generated cases

Read `hc docs protocol generators` first. A large input is built by a
function of the problem's `generator.py`, and the case only names the call:

- Write each kind of large input as its own function, taking the size as
  an argument (`n`), so that one function serves several cases. A
  hand-designed worst case (descending input, all equal values, a skewed
  tree, the answer at the very end) is a function that ignores `rng`; use
  `rng` only for random data, and never the `random` module.
- The input must satisfy the constraints and guarantees like any other,
  [by construction](#test-cases).
- To check that the inputs meet the constraints, pipe them into a script
  that prints only its verdict:
  `hc cases list <slug> --expand --json | python3 .hc/scratch/<slug>/check.py`
  (`--expand` runs generator.py as hc does, `--json` prints one case per
  line). Never print the expanded inputs themselves: a large input costs
  far more to read than to check. `hc_runtime` cannot be imported outside
  hc, so never stand in for `rng` with the `random` module either: you
  would check other inputs than the cases'.
- Add the cases with
  `hc cases add <slug> --generate <name> --args '{"n": 100000}' --seed 7`,
  or as `generate` lines in `cases.jsonl`, each with a `note` saying what
  it targets. Never write `digest`: `hc cases build` writes it.
- Keep generated cases for size: large inputs only. Small inputs stay
  stored, where a reader can see them.
- A generated case has no `expected`, so a problem judged with `any_of`
  has none: if it needs large inputs, it needs `unordered` or a checker.
- After changing `generator.py`, run `hc cases build <slug>` again: the
  changed inputs get new digests.

## Time limits

A run may take 40 times what the reference in its language takes on the
same cases, in total (that time counted as at least 1.25ms), timed on the
user's machine; beyond that it is Time Limit Exceeded
(`hc docs protocol time-limit`). The factor is hc's, not the problem's:
the cases decide whether a solution of a worse complexity fails.

- A problem tests complexity when the statement requires one, a naive
  solution exceeds about 10^8 operations at the constraint limits, or
  there is a well-known brute force. Then its large cases must make that
  slower solution exceed the limit with room to spare: about 80 times the
  reference, twice the factor, since timings vary.
- The ratio of the totals is the average of the ratios of the cases,
  weighted by the reference's time on each. Small cases weigh almost
  nothing: write as many as the edge cases need. The large cases decide
  the ratio.
- Make most large cases the slower solution's worst shape at the
  constraint limits (for a scan over pairs, the answer at the end; for a
  recursion without memoization, no early exit), varied in values and
  layout, so that they take most of the reference's time: the ratio of
  the totals is then close to their ratio. They need not outnumber the
  small cases.
- Stop adding large cases once the slower solution takes about 200 times
  the reference in total, or as much as its largest ratio on one case
  allows: more of them only slow every run down, the users' included.
- A large input the slower solution gets through quickly (its answer
  found early, an order it likes) adds to the reference's time, not to the
  gap, and pulls the ratio down. Give it the worst shape as well, unless it
  targets something only a large input shows (overflow, deep recursion),
  and keep such cases few. What a small input shows (an early answer, a
  length mismatch, a difference in the first letters) belongs in a small
  case.
- When the reference takes less than 1.25ms on all the cases (often in
  Go), the limit is 50ms whatever the ratio: the slower solution must
  spend that much in total, so give it enough worst-shape cases.
- No number of cases takes the ratio past the largest ratio on one case:
  if the worst shape at the limits stays under the gap, more cases cannot
  fix it.
- Check it: write the slower solution in `.hc/scratch/<slug>/`, and run
  `hc test <slug> --all --file .hc/scratch/<slug>/slow.py`, or on one
  candidate input `hc test <slug> --generate <name> --args '{"n": 100000}' --file .hc/scratch/<slug>/slow.py`.
  It must be Time Limit Exceeded, and the reference, run the same way with
  `--file reference/solution.py`, Accepted.
- When the constraints are too small for the gap (the slower solution
  stays within the factor at the largest input they allow), leave it:
  LeetCode accepts it too. Say so in the [summary](#summary).
- A problem that does not test complexity needs nothing more: the factor
  leaves room for any reasonable solution.

## Workflow: generate test cases

1. Read `problem.yaml`, `description.md` and the existing `cases.jsonl`.
   Note the constraints (ranges, lengths, uniqueness) and the guarantees.
   If the definition looks wrong, switch to `hc-problem`. `description.md`
   is the statement in the pack's default locale; the statements in other
   locales (`description.<locale>.md`) are not yours to read or write.
   `hc validate` errors about a missing locale are the user's to fix:
   report them, and never translate a statement or a title.
2. [Decide the judge rule](#decide-the-judge-rule) and write it to
   problem.yaml.
3. Write `reference/solution.py`, then a reference in every other
   language the pack is solved in, `reference/solution.go`
   ([Reference solutions](#reference-solutions)). If more than one output
   is acceptable, make the Go one return a different valid answer.
4. For `checker`, write `checker.py` ([Checkers](#checkers)).
5. Add the inputs to `cases.jsonl` ([Test cases](#test-cases)), and the
   large ones as [generated cases](#generated-cases).
6. Run `hc cases build <slug> --lang python` to compute `expected` with the
   Python reference and write the digests of the generated cases. With
   `any_of`, whose `expected` you wrote yourself, there is nothing to
   build.
7. Run `hc validate <slug> --run`. It must report no errors, and every
   reference must pass every case. Fix and repeat until it does.
8. [Prove the cases reject wrong solutions](#test-cases), and for
   `checker`, [that the checker rejects wrong answers](#checkers). If you
   added cases, run `hc cases build <slug> --lang python` again (with
   `any_of`, write their `expected` yourself). Then run
   `hc validate <slug> --run` once more with the real reference restored.
9. If the problem tests complexity, check that a slower solution exceeds
   the [time limit](#time-limits), and strengthen the large cases if it
   does not.
10. In a pack checkout (the workspace has `pack.yaml`), run `hc pack check`
   once you are done with every problem you were asked for: it checks the
   whole pack as its CI does, and must pass. The `hc-pack` skill says how
   a problem lands in the pack (groups, commits).
11. Write the [summary](#summary).

## Workflow: fix a test case

You are told the problem, the case numbers (1-based, counting only the
lines that hold a case) and the user's complaint.

1. Inspect the cases with `hc cases list <slug> --case N --verbose`, and
   re-read the constraints.
2. Decide what is wrong:
   - the input violates the constraints;
   - the expected output is wrong (a bug in the reference);
   - the judge rule is wrong (the order should not matter, several answers
     are valid), or `judge.compare` is unset: decide it as in
     [Decide the judge rule](#decide-the-judge-rule);
   - `checker.py` rejects a valid answer or accepts a wrong one;
   - a solution of the complexity the problem accepts is Time Limit
     Exceeded: the reference is hyper-optimized, or a case favors it; a
     slower one passes where the constraints allow a gap: the large cases
     miss its worst shape, or the reference in its language has a slow
     constant ([Time limits](#time-limits));
   - the definition is wrong (signature, structure inputs): fix it with
     `hc-problem`;
   - nothing: the case is right and the user's solution is wrong. Then say
     so and explain why, and change nothing.
3. Fix the root cause: the reference, the input, `judge.compare` or
   `checker.py`. When you loosen the judge rule, prove it with a second
   reference. When you change a checker, prove it again with wrong
   solutions.
4. Run `hc cases build <slug> --force --lang python` to recompute every
   expected output (with `any_of`, correct the arrays yourself instead),
   then `hc validate <slug> --run`.
5. Report what was wrong and what you changed.

## Summary

Report:

- the cases you added, counted by category;
- the wrong solutions you tried, and the case that rejects each;
- the judge rule, with the sentence of the statement it rests on;
- for a checker, what it verifies, and for a randomized problem its
  false-reject probability per case;
- what the Go reference does differently, when more than one output is acceptable;
- whether the problem tests complexity, and if so the slower solution you
  timed and its result, or why the constraints cannot separate it;
- anything the statement requires that hc does not judge.

## Useful commands

- `hc show <slug>`: the statement
- `hc cases list <slug> [--case N]`: inspect cases (`--expand --json`
  pipes generated inputs into a script)
- `hc cases build <slug> [--force] [--lang python]`: fill expected outputs from the reference
- `hc validate <slug> --run`: check the problem and run every reference on every case
- `hc pack check`: in a pack checkout, check the whole pack as its CI does
  (every problem valid, ready and passing `--run`); run it when you have
  finished the problems you were asked for
- `hc test <slug> --all --file <path>`: run a solution file, such as a
  slower one in `.hc/scratch/<slug>/`, against the time limit
- `hc test <slug> --lang python`: run the user's solution (only when diagnosing)
- `hc docs protocol <section>`: one section of the spec
