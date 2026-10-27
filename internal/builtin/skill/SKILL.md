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
  checker.py         the checker, only with judge.compare: checker
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
  compare: unordered        # exact | unordered | unordered_deep | any_of | checker — no default
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

### Round trips (class problems)

When a design problem asks for an encoder and a decoder in a format the
solution chooses, and the judge only checks that decoding the encoded value
gives it back (297 Serialize and Deserialize Binary Tree, 449 Serialize and
Deserialize BST, 535 Encode and Decode TinyURL), set `judge.round_trip`:

```yaml
kind: class
class:
  name: Codec
  constructor: []
  methods:
    - {name: serialize, params: [{name: root, type: TreeNode}], returns: string}
    - {name: deserialize, params: [{name: data, type: string}], returns: TreeNode}
judge:
  compare: exact
  round_trip: {encode: serialize, decode: deserialize, instances: separate}
```

- `instances` comes from the template's comment: `separate` when it encodes
  and decodes with two objects (`ser = Codec(); deser = Codec()`: 297, 449),
  `same` when one object does both (`codec.decode(codec.encode(url))`: 535,
  whose codec has to remember its mapping).
- Cases are not `ops`/`args`: `input` holds encode's parameter by name,
  `{"root": [1,2,null,3]}` or `{"longUrl": "https://leetcode.com/problems/design-tinyurl"}`,
  and `expected` is decode's result, filled by `hc cases build` (it equals the
  input for a correct codec).
- The judge rule is almost always `exact`. Requirements on the encoded
  string itself, such as 449's "as compact as possible", are not judged;
  say so in your summary rather than inventing a check.
- The constructor takes no parameters, encode takes one value and returns
  `string`, decode takes that `string` and returns the value's type.

### Structure inputs (function problems only)

When the statement says the judge builds a structure from inputs the
solution does not receive ("`pos` is not passed as a parameter", "the judge
creates the linked structure from `skipA`/`skipB`", "`p` and `q` are nodes of
the tree", "you are given the node to delete, not the head"), describe it
instead of passing the raw value:

```yaml
function:
  name: getIntersectionNode
  params:
    - {name: headA, type: ListNode}
    - {name: headB, type: ListNode, join: {list: headA, skip: skipB, at: skipA}}
  hidden:                     # case inputs never passed to the solution
    - {name: skipA, type: integer}
    - {name: skipB, type: integer}
  returns: ListNode
  returns_ref: headA          # the returned node belongs to headA
```

- `hidden`: extra inputs (`{name, type}`); every one must be used by one of
  the attributes below or by `judge.output`.
- `cycle: pos` (ListNode param): the tail links to the node at index `pos`, an
  integer hidden input; `-1` = no cycle (141, 142). Case: `{"head": [3,2,0,-4], "pos": 1}`.
- `join: {list: A, skip: s, at: a}` (ListNode param B): B keeps its first `s`
  nodes, then continues with A's nodes from index `a` (160). In cases, A and B
  are the full lists including the shared tail, so `B[s:] == A[a:]`; no
  intersection: `a == len(A)` and `s == len(B)`.
- `ref: X` (ListNode/TreeNode param): the case gives a node **value** (an
  integer), the solution gets the node of structure X holding it (235/236
  `p`, `q` with `ref: root`; 237 `node` with `ref: head` where `head` is a
  hidden ListNode and `judge.output: head`). The value must occur exactly once in X.
- `copy: X` (ListNode/TreeNode param): not in cases; the solution gets a deep
  copy of param X (1379 `cloned`).
- `returns_ref: X` when the function returns a node of structure X. Expected
  is the node's 0-based **index** in a ListNode X, its **value** in a TreeNode
  X, and `null` for no node (142: `"expected": 1`; 236: `"expected": 3`).
  Returning a node outside X is a runtime error.

`params` must match LeetCode's template (the solution's real signature);
hidden inputs never reach the solution. Keep the input names the statement
uses (`pos`, `skipA`, `skipB`), and drop inputs the driver does not need
(160's `intersectVal`). A fetched problem may declare such inputs as plain
parameters (236: `p` and `q` as `integer`, while `templates/solution.go`
takes `*TreeNode`); fix `problem.yaml` to the template's signature with
these attributes. Design problems cannot use them.

### LeetCode's Node types

LeetCode calls six structures `Node`; problem.yaml names the one the template
means by its fields (one per problem, never in arrays, no structure
attributes). Solution code still calls it `Node`: Go predefines it, Python
references start with `from hc_runtime import GraphNode as Node` (etc.).
Case values and `expected` use these forms:

| Fields in the template | Type | Value |
|---|---|---|
| `Neighbors` | `GraphNode` | adjacency list of nodes 1..n (node i has value i), param = node 1: `[[2,4],[1,3],[2,4],[1,3]]`, `[[]]` one node, `[]` none. Write symmetric lists without self loops when the statement says undirected |
| `Children` | `NaryNode` | level order, `null` after each node's children: `[1,null,3,2,4,null,5,6]` |
| `Next`, `Random` | `RandomListNode` | `[[val, randomIndex or null], ...]` |
| `Left`, `Right`, `Next` | `NextTreeNode` | input: TreeNode level order; expected: levels read through next pointers, `[[1],[2,3],[4,5,7]]` (not LeetCode's `[1,#,2,3,#,...]`) |
| `IsLeaf`, `TopLeft`, ... | `QuadNode` | level order of `[isLeaf, val]` (0/1), four child slots per node: `[[0,1],[1,0],[1,1],[1,1],[1,0]]`; a non-leaf is always output as `[0,1]` |
| `Prev`, `Next`, `Child` | `MultilevelNode` | LeetCode's serialization: `[1,2,3,4,5,6,null,null,null,7,8,9,10,null,null,11,12]` |

Let `hc cases build` compute `expected` for these as for any other type.
When the function returns a deep copy of a parameter (133 Clone Graph, 138
Copy List with Random Pointer), add `returns_copy: <param>` to `function`:
returning any original node is then a runtime error; `hc fetch` never sets
it.

### NestedInteger and Employee

Also judge-defined, under LeetCode's names (Python references import them:
`from hc_runtime import NestedInteger`):

- `NestedInteger`: an integer or an array of NestedInteger values; type
  `"NestedInteger[]"` for a list of them (341 constructor arg
  `[[1,1],2,[1,1]]`). A function may return one (385: `"expected":
  [123,[456,[789]]]` or `324`). Iterator problems (341) are class problems:
  write `ops` as the statement's test loop, `hasNext` then `next` until
  `hasNext` is false. `hc fetch` drops their examples, which give only the
  constructor's input; write them as `example: true` cases.
- `Employee`: `[id, importance, [subordinate ids]]`; `"Employee[]"` needs
  distinct ids and known subordinates (690).

Types: `integer long double boolean string character ListNode TreeNode void`,
the Node types, `NestedInteger`, `Employee`, arrays with `[]` suffix (up to 3
levels; ListNode, TreeNode, NestedInteger and Employee arrays 1 level; no
Node arrays).
Quote array types inside `{...}` / `[...]` (`type: "integer[]"`): unquoted,
`[]` is YAML syntax. Unknown fields are errors.

### cases.jsonl

One object per line: `{"input": {...}, "expected": <json>, "example": true, "note": "..."}`.

- Function problems: `input` maps every parameter and hidden input name to
  its value (a `ref` parameter takes the node's value; leave out `copy`
  parameters).
- Class problems: `input` is `{"ops": ["LRUCache","put","get"], "args": [[2],[1,1],[1]]}`
  and `expected` is `[null,null,1]` (null for the constructor and void methods).
  Round trips: `input` is `{"root": [1,2,3]}` (encode's parameter), `expected`
  decode's result.
- ListNode is an array `[1,2,3]`; TreeNode is LeetCode level order `[3,9,20,null,null,15,7]`;
  character is a 1-char string `"a"`.
- `expected` may be omitted; `hc cases build` fills it from the reference solution.
- `any_of` mode: `expected` is an array of all acceptable answers.
- `checker` mode: `expected` is the reference's answer, filled by `hc cases build`.
- `example: true` marks the statement examples (run by `hc test`); `hc submit` runs all.
- `hc validate` checks every value against its declared type, rejects unknown
  inputs or case fields, requires each op's exact argument count, and checks
  the shape of `expected`.

### Checkers

`judge.compare: checker` hands the decision to `checker.py` in the problem
directory (required with this mode, an error with any other). It is Python
for solutions in every language and must run on Python 3.10:

```python
def check(input, output, expected):
    """Return None or True to accept; False or a message string to reject."""
```

- `input`: the case input as decoded JSON (the parameters and hidden inputs
  of a function problem; `{"ops": [...], "args": [...]}` for a class problem;
  encode's parameter for a round trip).
- `output`: the solution's output as decoded JSON, in the form of `expected`.
- `expected`: the reference's answer from `hc cases build` — one valid answer
  among many. Use it for the optimal value instead of solving the problem
  again: the longest palindrome's length, the size of the largest subset, the
  minimum sum. Check validity from `input` and optimality against `expected`.
- Return a message that says what is wrong (`"nums[3] = 2 is not less than
  nums[4] = 2"`); users see it with the Wrong Answer.
- `from hc_runtime import decode, TreeNode` rebuilds structures:
  `decode("TreeNode", output)`. Plain JSON is enough for most checkers.
- A checker that raises, returns anything else or runs out of time (it gets
  `judge.time_limit`) fails the whole run as a broken problem. Validate
  `output`'s type and shape before using it, so a wrong output is a
  rejection, not a crash.

Randomized problems (Shuffle an Array, Random Pick with Weight, Linked List
Random Node, `getRandom`) are class problems whose cases make many calls —
hundreds to a few thousand. The checker checks every output's validity, then
the distribution: compare observed with expected counts per bucket (value,
position, index) with generous bounds — a chi-square test at a tiny
significance level, or at least 5 standard deviations per bucket — so a
correct solution essentially never fails while an obviously non-random one
(always the same pick, the identity shuffle) does. Write the false-reject
probability per case in a comment of the checker, and keep it below 1e-6.
`expected` of the random calls is just one sample: only compare
deterministic calls with it (`reset`).

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
   - many valid answers ("return any of them", "any valid order", any
     height-balanced BST), or a randomized output → `checker`, and write
     `checker.py` (see [Checkers](#checkers)). Prefer a built-in mode when one
     fits;
   - in-place problems: `judge.output` (and `output_length: return` when only
     the first k elements count) come from LeetCode's metadata — keep them;
     they combine with any mode, `checker` included.
3. Write a **reference solution** in `reference/solution.py` (it must run on
   Python 3.10, the oldest version hc supports; same shape as LeetCode:
   `class Solution` with the method, or the design class). Annotate with
   `list[int]` and `TreeNode | None`, and if needed start the file with
   `from hc_runtime import ListNode, TreeNode`.
   Prefer a simple, obviously correct algorithm. If it is too slow for the
   largest inputs, also write a brute force and cross-check both on small
   random inputs before trusting the fast one.
4. **If more than one output is acceptable, prove the rule**: also write
   `reference/solution.go` so that it returns a *different* valid answer than
   the Python one wherever the rule allows (another order, another valid
   answer). `hc validate --run` runs both against the same expected outputs,
   so a rule that is too strict fails there. For a randomized problem, the
   two references may use different (correct) random algorithms. A Go reference starts with
   `package main` (LeetCode's template in `templates/solution.go` omits it)
   and must build with Go 1.22: no newer language features or standard
   library functions (e.g. `slices.Sorted`, `maps.Keys`).
   **Prove a checker rejects wrong answers** too: temporarily replace
   `reference/solution.go` (keep a copy) with plausible wrong solutions — an
   invalid answer, a valid but not optimal one, always the same random pick —
   and check that `hc validate <slug> --run` reports Wrong Answer with a
   helpful checker message on the cases you expect. Then restore the real
   reference.
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
   For a randomized problem, a few cases with many calls (around 1000–3000)
   are better than many small ones: the checker needs samples.
6. Run `hc cases build <slug> --lang python` to compute `expected` with the
   reference (for `checker` too: the checker receives it). For `any_of`, write the `expected` arrays yourself (compute
   every acceptable answer, e.g. with a throwaway script); `hc cases build`
   refuses to fill them, because a reference finds only one answer.
7. Run `hc validate <slug> --run` — it must report no errors, and every
   reference solution must pass every case.
8. Summarize what you added (count by category) and the judge rule you chose,
   with the sentence of the statement it rests on; for a checker, what it
   verifies (and its false-reject probability for a randomized problem).

## Workflow: fix a test case

You will be told the problem, the case number(s) (1-based line among
non-comment lines; see `hc cases list <slug>`) and the user's complaint.

1. Inspect the case with `hc cases list <slug> --case N` and re-read the
   constraints.
2. Decide what is wrong: the input violates constraints, the expected output
   is wrong (reference bug), the judge rule is wrong (e.g. order should not
   matter; see step 2 of the generate workflow — and if `judge.compare` is
   unset, decide it), or `checker.py` rejects a valid answer or accepts a
   wrong one. It is also possible the case is right and the user's solution is
   wrong — then say so and explain why, without changing anything.
3. Fix the root cause: correct the reference solution, the input,
   `judge.compare` or `checker.py`. When you loosen the judge rule, prove it with a second
   reference as in step 4 of the generate workflow. Then run `hc cases build <slug> --force` to recompute every
   expected output and `hc validate <slug> --run`.
4. Report what was wrong and what you changed.

## Useful commands

- `hc show <slug>` — statement
- `hc cases list <slug> [--case N]` — inspect cases
- `hc cases build <slug> [--force] [--lang python]` — fill expected outputs from the reference
- `hc validate <slug> --run` — check protocol and run every reference on all cases
- `hc test <slug> --lang python` — run the user's solution (only when diagnosing)
