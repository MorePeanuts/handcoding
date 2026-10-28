---
name: hc-cases
description: Decide how an hc (handcoding) problem is judged and write its test cases. Use when generating or fixing the test cases (cases.jsonl) of a problem in an hc workspace, choosing its judge rule (judge.compare in problem.yaml), writing or fixing a reference solution (reference/solution.py or .go), or writing a checker (checker.py); `hc gen` and `hc fix` use it. For the problem's definition itself (signature, structure inputs, statement, templates), use hc-problem instead.
---

# handcoding test case authoring

`hc` is a local judge for LeetCode-style problems. A workspace contains
`hc.yaml`; problems live in `problems/<dir>/`, the user's own code in
`solutions/<lang>/<dir>/`. **Never read or modify the user's solutions
unless asked to diagnose a reported failure, and never change them.**

This skill covers how a problem is judged: its judge rule, its reference
solutions, its checker and its test cases. What the problem is (directory,
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
  `any_of`;
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
  `judge.tolerance` to it.
- In-place problems: `judge.output` and `output_length` belong to the
  definition and combine with every mode, `checker` included. Keep them
  (`hc docs protocol in-place-output`).
- `judge.timeout` guards against hangs, not slow solutions
  (`hc docs protocol timeout`). Leave it unset; if the reference needs a
  large part of the default on your largest case, make the case smaller.

## Reference solutions

Read `hc docs protocol solution-abi`, and `hc docs protocol python` or
`hc docs protocol go` for the language you write. Expected outputs come
from running the reference, never from you, so the reference must be
right.

- Write `reference/solution.py` first. It must run on Python 3.10, the
  oldest version hc supports (`hc docs protocol language-versions`); import
  only the public names of `hc_runtime` and the standard library.
- Prefer a simple, obviously correct algorithm. If it is too slow for the
  largest inputs, also write a brute force and cross-check the two on small
  random inputs with a throwaway script before you trust the fast one.
- A Go reference starts with `package main` (LeetCode's Go template omits
  it) and must build with Go 1.22: no newer language features or standard
  library functions (`slices.Sorted`, `maps.Keys`, range over a function).
  `hc validate --run` builds it with the installed Go, which is newer and
  does not catch them, so stay with what you know Go 1.22 has.

**Prove the rule with a second reference.** When more than one output is
acceptable, also write `reference/solution.go` so that it returns a
*different* valid answer than the Python one wherever the rule allows:
another order, another valid answer, another (correct) random algorithm.
`hc validate --run` runs both against the same expected outputs, so a rule
that is too strict fails there.

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
- Aim for 15 to 40 cases covering:
  - every statement example (`"example": true`);
  - minimum sizes and empty inputs the constraints allow;
  - boundary values (minimum and maximum values, negative numbers,
    duplicates);
  - tricky structures (skewed trees, one node, all equal, sorted and
    reverse sorted; for structure inputs, no cycle, no intersection, a
    node at the root or the tail);
  - a few large inputs near the constraint limits, to catch quadratic
    solutions.
- Every input must satisfy the stated constraints and guarantees ("exactly
  one solution", "all values are unique").
- Generate large inputs with a throwaway script inside the workspace (you
  cannot write outside it), and delete it afterwards. Keep cases.jsonl
  under about 2 MB.
- Give each non-example case a `note` saying what it targets.
- Randomized problems: a few cases with many calls (1000 to 3000) are
  better than many small ones; the checker needs samples.

## Workflow: generate test cases

1. Read `problem.yaml`, `description.md` and the existing `cases.jsonl`.
   Note the constraints (ranges, lengths, uniqueness) and the guarantees.
   If the definition looks wrong, switch to `hc-problem`.
2. [Decide the judge rule](#decide-the-judge-rule) and write it to
   problem.yaml.
3. Write `reference/solution.py` ([Reference solutions](#reference-solutions)).
4. If more than one output is acceptable, write the second reference in
   `reference/solution.go`. For `checker`, write `checker.py`
   ([Checkers](#checkers)).
5. Add the inputs to `cases.jsonl` ([Test cases](#test-cases)).
6. Run `hc cases build <slug> --lang python` to compute `expected` with the
   Python reference. Skip this step with `any_of`, whose `expected` you
   wrote yourself.
7. Run `hc validate <slug> --run`. It must report no errors, and every
   reference must pass every case. Fix and repeat until it does.
8. For `checker`, [prove it rejects wrong answers](#checkers), and run
   `hc validate <slug> --run` once more with the real reference restored.
9. Write the [summary](#summary).

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
- the judge rule, with the sentence of the statement it rests on;
- for a checker, what it verifies, and for a randomized problem its
  false-reject probability per case;
- what the second reference does differently, if there is one;
- anything the statement requires that hc does not judge.

## Useful commands

- `hc show <slug>`: the statement
- `hc cases list <slug> [--case N]`: inspect cases
- `hc cases build <slug> [--force] [--lang python]`: fill expected outputs from the reference
- `hc validate <slug> --run`: check the problem and run every reference on every case
- `hc test <slug> --lang python`: run the user's solution (only when diagnosing)
- `hc docs protocol <section>`: one section of the spec
