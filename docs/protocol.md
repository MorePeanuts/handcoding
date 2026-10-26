# handcoding protocols

English · [简体中文](protocol_zh.md)

This document defines two protocols:

1. **Problem protocol**: how a problem in the bank is laid out on disk. Use it to write your own problems (`hc new`, `hc import`).
2. **Driver protocol**: the contract between hc and the per-language test drivers. Use it to add a language.

---

## 1. Problem protocol

### Writing a problem by hand

```sh
hc new sum-array --id 9001            # scaffold problems/9001-sum-array/ (--kind class for a design problem)
# edit problem.yaml and description.md, write cases.jsonl inputs and reference/solution.py
hc cases build sum-array              # compute "expected" by running the reference
hc validate sum-array --run           # check everything; every reference must pass every case
hc test sum-array                     # now solve it like any other problem
```

A complete function problem looks like this:

```
problems/9001-sum-array/
  problem.yaml
  description.md
  cases.jsonl
  reference/solution.py
```

```yaml
# problem.yaml
id: "9001"
slug: sum-array
title: Sum Array
difficulty: easy
tags: [array]
kind: function
function:
  name: sumArray
  params:
    - name: nums
      type: integer[]
  returns: long
judge:
  compare: exact
```

```json
{"input": {"nums": [1,2,3]}, "example": true}
{"input": {"nums": [-5]}, "note": "single negative"}
{"input": {"nums": [1000000000,1000000000,1000000000]}, "note": "overflows int32"}
```

```python
# reference/solution.py
from typing import List


class Solution:
    def sumArray(self, nums: List[int]) -> int:
        return sum(nums)
```

`hc cases build sum-array` then adds `"expected": 6`, `-5` and `3000000000` to the three lines. You can also write `expected` yourself; `hc validate --run` checks it against the reference either way.

A class (design) problem replaces `function` with `class`. Each case is a sequence of operations:

```yaml
id: "9003"
slug: min-stack-lite
title: Min Stack Lite
kind: class
class:
  name: MinStack
  constructor: []
  methods:
    - name: push
      params:
        - name: val
          type: integer
      returns: void
    - name: pop
      params: []
      returns: void
    - name: getMin
      params: []
      returns: integer
judge:
  compare: exact
```

```json
{"input": {"ops": ["MinStack","push","push","getMin","pop","getMin"], "args": [[],[-2],[-3],[],[],[]]}, "expected": [null,null,null,-3,null,-2], "example": true}
```

```python
# reference/solution.py
class MinStack:
    def __init__(self):
        self.stack = []

    def push(self, val: int) -> None:
        low = min(val, self.stack[-1][1]) if self.stack else val
        self.stack.append((val, low))

    def pop(self) -> None:
        self.stack.pop()

    def getMin(self) -> int:
        return self.stack[-1][1]
```

### Layout

```
problems/0001-two-sum/          directory name: <4-digit number>-<slug> when id is a number, otherwise <slug>
  problem.yaml                  metadata, signature and judge options (required)
  description.md                statement, in Markdown
  assets/rev1ex1.jpg            images used by the statement (optional), written as ![](assets/rev1ex1.jpg)
  cases.jsonl                   test cases, one JSON object per line
  reference/solution.py         reference solutions (optional, any languages), used to compute expected outputs
  reference/solution.go
  templates/solution.go         starter code (optional; generated from the signature if missing)
  templates/solution.py
```

`hc import` copies only these entries; anything else in the directory (editor files, a `solutions/` folder, ...) is left behind.

### problem.yaml

| Field | Meaning |
|---|---|
| `id` | Problem ID (a string, optional), used for sorting and lookup (`hc test 9001`). Must be unique in the bank. A number also prefixes the directory name |
| `slug` | Unique identifier in kebab-case: letters, digits and single hyphens, preferably lower case (required) |
| `title` | Title (required) |
| `difficulty` | `easy`, `medium` or `hard` |
| `tags` | List of tags |
| `kind` | `function` (one function) or `class` (a design problem). Defaults to `class` when there is a `class` section, otherwise `function` |
| `function` | Signature of a function problem: `name`, `params` (a list of `{name, type}`), `returns` (defaults to `void`) |
| `class` | Signature of a design problem: `name`, `constructor` (a list of `{name, type}`), `methods` (a list of `{name, params, returns}`) |
| `judge.compare` | How outputs are compared (see below). No default: a problem without it is *not ready* (see below) |
| `judge.tolerance` | Floating-point tolerance. Defaults to `1e-5` when returning `double`; either the absolute or the relative error must be within it |
| `judge.output` | For in-place problems: the judge compares this parameter's value after the call instead of the return value |
| `judge.output_length` | `return`: judge only the first k elements of `judge.output`, k being the (integer) return value. LeetCode's "remove duplicates" problems |
| `judge.time_limit` | Time limit for a whole run, as a duration such as `5s` or `500ms`. Defaults to `timeout` in hc.yaml |
| `source` | Where the problem comes from: `site`, `url` |

Unknown fields are errors, so a typo such as `judgee:` or `parms:` is reported instead of silently falling back to the defaults.

> **YAML pitfall.** Inside the compact `{...}` / `[...]` syntax, `[]` is YAML syntax, so array types must be quoted there: `{name: nums, type: "integer[]"}`. In block style (`type: integer[]` on its own line, as in the examples above) quotes are optional.

**Not ready.** `judge.compare` must be written out, even for `exact`. A problem without it has no decided judge rule: `hc submit` refuses it, `hc test` shows the outputs next to the expected ones without a verdict ("Not Judged"), and `hc validate` warns. `hc fetch` leaves problems in this state on purpose, because LeetCode does not publish how answers are judged; `hc gen` decides the rule from the statement, or you set it by hand.

An in-place problem returns `void` and names the parameter to judge:

```yaml
function:
  name: rotate
  params:
    - name: nums
      type: integer[]
    - name: k
      type: integer
  returns: void
judge:
  output: nums      # expected is the value of nums after the call
```

When the function returns a length k and only the first k elements count ("the elements after the first k do not matter"):

```yaml
function:
  name: removeDuplicates
  params:
    - name: nums
      type: integer[]
  returns: integer
judge:
  compare: exact
  output: nums
  output_length: return   # judge nums[:k], where k is the return value
```

Comparison modes:

| `compare` | Meaning | Examples |
|---|---|---|
| `exact` | The JSON values are equal (numbers compare by value) | Most problems |
| `unordered` | The outermost array is a multiset; order is ignored | Two Sum, Permutations |
| `unordered_deep` | Order is ignored at every array level | Group Anagrams, Subsets |
| `any_of` | `expected` is an array of all acceptable answers | Problems with a small set of valid answers |

`unordered` and `unordered_deep` need an array result.

### Types

Type names are LeetCode's `metaData` names, so problems fetched from LeetCode need no translation:

| Type | JSON | Go | Python |
|---|---|---|---|
| `integer` / `long` | whole number | `int` / `int64` | `int` |
| `double` | number | `float64` | `float` |
| `boolean` | `true` / `false` | `bool` | `bool` |
| `string` | string | `string` | `str` |
| `character` | one-character string `"a"` | `byte` | `str` |
| `ListNode` | array `[1,2,3]` | `*ListNode` | `Optional[ListNode]` |
| `TreeNode` | level-order array `[3,9,20,null,null,15,7]` | `*TreeNode` | `Optional[TreeNode]` |
| `T[]` | array, up to 3 levels; 1 level for node arrays | `[]T` | `List[T]` |
| `void` | return type only | — | `None` |

`list<T>` is normalized to `T[]`, and `int`, `bool`, `char` and `float` to `integer`, `boolean`, `character` and `double`. Special types such as `Node` (graphs, N-ary trees, lists with random pointers) and `NestedInteger` are not supported yet.

### cases.jsonl

One case per line:

```json
{"input": {"nums": [2,7,11,15], "target": 9}, "expected": [0,1], "example": true}
{"input": {"nums": [-1,-2,-3,-4,-5], "target": -8}, "expected": [2,4], "note": "negative numbers"}
```

- `input`: for function problems, an object with exactly one value per parameter, keyed by name. For design problems, `{"ops": [...], "args": [[...], ...]}`; `ops[0]` must be the class name (the constructor), and each `args` entry holds exactly the arguments of its op.
- `expected`: the expected output. For design problems, an array with each operation's return value, using `null` for the constructor and `void` methods. For `any_of`, an array of acceptable answers. If missing, `hc cases build` computes it with the reference solution.
- `example`: the case is an example from the statement. `hc test` runs only these by default (the first 3 cases if none is marked).
- `note`: a description of the case.
- No other fields are allowed. Blank lines and lines starting with `//` are ignored. Cases are numbered from 1, counting only the lines that hold a case.

### Reference solutions and templates

A reference solution is written exactly like a user's solution, in `reference/solution.py` or `reference/solution.go`. `hc start` shows the expected shape: the generated starter is a reference with the body missing.

| | Function problem | Class problem |
|---|---|---|
| Python | `class Solution` with a method named `function.name`, or a module-level function of that name | A class named `class.name`; `__init__` takes the constructor parameters; methods have the names in `methods` |
| Go | `package main`; `func <function.name>(...)` with the Go types from the table above | `type <Name> struct`, `func Constructor(...) <Name>`, and methods with pointer receivers whose names start with an upper-case letter (`getMin` → `GetMin`) |

- `ListNode` and `TreeNode` are provided: in Python, `from hc_runtime import ListNode, TreeNode`; in Go they are predefined (`Val`, `Next`, `Left`, `Right`), so don't declare them.
- With both references present, `hc cases build` uses Go, then Python; pick one with `-l python`. `hc validate --run` runs every reference.
- `templates/solution.go` and `templates/solution.py` replace the generated starter code. They must keep the signature, because the driver calls it.

### Validation

```
hc validate <slug>          # check against the protocol
hc validate <slug> --run    # also run every reference solution on every case; all must pass
```

`hc validate` reports, with the case number and field:

- `problem.yaml`: syntax errors (with line and column), unknown fields, invalid slug, missing title, unsupported types, a `judge.output` that is not a parameter, a `void` function without `judge.output`, a misused `judge.output_length`, unknown `judge.compare`, an invalid `judge.time_limit`;
- `cases.jsonl`: invalid JSON, unknown fields, missing or unknown inputs, values that don't match their declared type (`input "nums": [1]: must be integer, got string "x"`), wrong argument counts and unknown methods in design problems, and an `expected` of the wrong shape;
- with `--run`: reference solutions that don't compile, crash, or disagree with `expected` (both values are shown).

It warns about an undecided judge rule (no `judge.compare`), cases without `expected`, no case marked as an example, no reference solution, missing statement images and an ID used by another problem.

---

## 2. Driver protocol (adding a language)

A language implements the `internal/lang.Language` interface and is registered in `internal/app.Languages`:

```go
type Language interface {
    ID() string                       // "go"; used in paths and --lang
    Name() string
    Aliases() []string
    SolutionFile() string             // the file the user edits, e.g. solution.go
    Starter(p *problem.Problem) (string, error)            // starter code
    Prepare(dir string, p *problem.Problem) error          // writes driver/runtime files (called before every run; must be idempotent)
    Build(ctx context.Context, dir, binDir string) error   // compiles or checks syntax; returns *lang.CompileError on failure
    Command(ctx context.Context, dir, binDir string) *exec.Cmd
    DebugCommand(dir, binDir string, p *problem.Problem, opts lang.DebugOptions) (*exec.Cmd, error)
    Check() error                     // whether the toolchain is available
}
```

A driver only **decodes the input, calls the user's code and encodes the output**. It never judges. All judging happens in Go (`internal/judge`), so a new language doesn't reimplement comparison.

- `HC_CASES` names a JSON file containing `[{"id": 1, "input": {...}}, ...]`.
- `HC_OUT` names a file the driver appends results to, as JSON Lines, one per case:

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  A non-empty `error` means a runtime error; keep only the stack frames from the user's code if you can. `elapsed_ns` covers only the time spent in the user's code.
- The user's stdout and stderr go straight to the terminal, so print debugging works.
- If the driver crashes or times out, cases without a result line are judged Runtime Error or Time Limit Exceeded respectively.

The existing drivers are good references: `internal/lang/golang` (generated code plus a static runtime) and `internal/lang/python` (a dynamic runtime with the signature embedded).
