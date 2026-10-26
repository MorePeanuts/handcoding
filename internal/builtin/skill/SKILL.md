---
name: handcoding
description: Generate, verify and fix test cases for problems in a handcoding (hc) workspace — a local LeetCode-style judge. Use when asked to create test cases for an hc problem, to fix a wrong test case, to write a reference solution, or to author/import a custom problem in the hc problem format.
---

# handcoding test case authoring

`hc` is a local judge for LeetCode-style problems. A workspace contains
`hc.yaml`; problems live in `problems/<dir>/`, the user's own code in
`solutions/<lang>/<dir>/`. **Never read or modify the user's solutions unless
asked to diagnose a reported failure — and never change them.**

## Problem layout

```
problems/0001-two-sum/
  problem.yaml       metadata + signature + judge options
  description.md     statement
  assets/            optional images used by the statement: ![](assets/x.png)
  cases.jsonl        test cases, one JSON object per line
  reference/         reference solutions (solution.py and/or solution.go)
  templates/         optional starter code (solution.go, solution.py)
```

### problem.yaml

```yaml
id: "1"
slug: two-sum
title: Two Sum
difficulty: easy            # easy | medium | hard
tags: [array, hash-table]
kind: function              # function | class
function:
  name: twoSum
  params:
    - {name: nums, type: "integer[]"}
    - {name: target, type: integer}
  returns: "integer[]"
judge:
  compare: unordered        # exact | unordered | unordered_deep | any_of — no default
  # tolerance: 1e-5         # floats; default 1e-5 when returning double
  # output: nums            # in-place problems: judge this param after the call (returns: void)
  # output_length: return   # judge only output[:k], k being the return value
```

A problem without `judge.compare` is **not ready**: its judge rule has not
been decided, `hc submit` refuses it and `hc test` shows outputs without a
verdict. `hc fetch` leaves it unset on purpose; deciding it is your job.

Class (design) problems:

```yaml
kind: class
class:
  name: LRUCache
  constructor: [{name: capacity, type: integer}]
  methods:
    - {name: get, params: [{name: key, type: integer}], returns: integer}
    - {name: put, params: [{name: key, type: integer}, {name: value, type: integer}], returns: void}
```

Types: `integer long double boolean string character ListNode TreeNode void`,
arrays with `[]` suffix (up to 3 levels, node arrays 1 level).
Quote array types inside `{...}` / `[...]` (`type: "integer[]"`): unquoted,
`[]` is YAML syntax. Unknown fields are errors.

### cases.jsonl

One object per line: `{"input": {...}, "expected": <json>, "example": true, "note": "..."}`.

- Function problems: `input` maps every parameter name to its value.
- Class problems: `input` is `{"ops": ["LRUCache","put","get"], "args": [[2],[1,1],[1]]}`
  and `expected` is `[null,null,1]` (null for the constructor and void methods).
- ListNode is an array `[1,2,3]`; TreeNode is LeetCode level order `[3,9,20,null,null,15,7]`;
  character is a 1-char string `"a"`.
- `expected` may be omitted; `hc cases build` fills it from the reference solution.
- `any_of` mode: `expected` is an array of all acceptable answers.
- `example: true` marks the statement examples (run by `hc test`); `hc submit` runs all.
- `hc validate` checks every value against its declared type, rejects unknown
  inputs or case fields, requires each op's exact argument count, and checks
  the shape of `expected`.

## Workflow: generate test cases

1. Read `problem.yaml`, `description.md` and existing `cases.jsonl`. Note the
   constraints carefully (ranges, lengths, uniqueness, guarantees like
   "exactly one solution").
2. **Decide the judge rule** — what counts as a correct output — and write it
   to `problem.yaml`, even when it is `exact`. Read the statement, not just
   the examples:
   - exactly one correct output → `exact` (with `tolerance` for doubles);
   - "return the answer in any order", "the order … does not matter" →
     `unordered` if only the outer list is free, `unordered_deep` if inner
     lists are free too (subsets, groups: yes; permutations: no — each
     permutation's own order matters);
   - several valid answers that can all be listed (a handful per case) →
     `any_of`, with `expected` holding every acceptable answer;
   - in-place problems: `judge.output` (and `output_length: return` when only
     the first k elements count) come from LeetCode's metadata — keep them.
   If no rule fits (e.g. "return any valid answer" with many possible
   answers, or a randomized problem), leave `judge.compare` unset, still
   write the reference and cases, and say in your summary that the problem
   needs a custom checker, which hc does not support yet.
3. Write a **reference solution** in `reference/solution.py` (Python 3.8
   compatible; same shape as LeetCode: `class Solution` with the method, or
   the design class). Start the file with
   `from typing import List, Optional` and, if needed,
   `from hc_runtime import ListNode, TreeNode`.
   Prefer a simple, obviously correct algorithm. If it is too slow for the
   largest inputs, also write a brute force and cross-check both on small
   random inputs before trusting the fast one.
4. **If more than one output is acceptable, prove the rule**: also write
   `reference/solution.go` so that it returns a *different* valid answer than
   the Python one wherever the rule allows (another order, another valid
   answer). `hc validate --run` runs both against the same expected outputs,
   so a rule that is too strict fails there. A Go reference starts with
   `package main` (LeetCode's template in `templates/solution.go` omits it).
5. Add inputs to `cases.jsonl` **without** `expected`. Keep existing lines
   (do not drop `example: true` cases). Aim for 15–40 cases covering:
   - every statement example (`"example": true`)
   - minimum sizes / empty inputs allowed by the constraints
   - boundary values (min/max values, negative numbers, duplicates)
   - tricky structures (skewed trees, single node, all equal, sorted/reverse sorted)
   - a few large inputs near the constraint limits to catch O(n²) solutions —
     generate them with a throwaway script inside the workspace (you cannot
     write outside it), and delete the script afterwards; keep cases.jsonl
     under ~2 MB
   Every input must satisfy the stated constraints and guarantees. Use
   `note` to say what each non-example case targets.
6. Run `hc cases build <slug> --lang python` to compute `expected` with the
   reference. For `any_of`, write the `expected` arrays yourself.
7. Run `hc validate <slug> --run` — it must report no errors, and every
   reference solution must pass every case.
8. Summarize what you added (count by category) and the judge rule you chose,
   with the sentence of the statement it rests on.

## Workflow: fix a test case

You will be told the problem, the case number(s) (1-based line among
non-comment lines; see `hc cases list <slug>`) and the user's complaint.

1. Inspect the case with `hc cases list <slug> --case N` and re-read the
   constraints.
2. Decide what is wrong: the input violates constraints, the expected output
   is wrong (reference bug), or the judge rule is wrong (e.g. order should not
   matter; see step 2 of the generate workflow — and if `judge.compare` is
   unset, decide it). It is also possible the case is right and the user's solution is
   wrong — then say so and explain why, without changing anything.
3. Fix the root cause: correct the reference solution, the input, or
   `judge.compare`. When you loosen the judge rule, prove it with a second
   reference as in step 4 of the generate workflow. Then run `hc cases build <slug> --force` to recompute every
   expected output and `hc validate <slug> --run`.
4. Report what was wrong and what you changed.

## Useful commands

- `hc show <slug>` — statement
- `hc cases list <slug> [--case N]` — inspect cases
- `hc cases build <slug> [--force] [--lang python]` — fill expected outputs from the reference
- `hc validate <slug> --run` — check protocol and run every reference on all cases
- `hc test <slug> --lang python` — run the user's solution (only when diagnosing)
