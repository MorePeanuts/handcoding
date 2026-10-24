# handcoding protocols

This document defines two protocols:

1. **Problem protocol**: how a problem in the bank is laid out on disk. Use it to write your own problems (`hc new`, `hc import`).
2. **Driver protocol**: the contract between hc and the per-language test drivers. Use it to add a language.

---

## 1. Problem protocol

### Layout

```
problems/0001-two-sum/          directory name: <4-digit number>-<slug>, or <slug> without a number
  problem.yaml                  metadata, signature and judge options (required)
  description.md                statement, in Markdown
  assets/rev1ex1.jpg            images used by the statement (optional), written as ![](assets/rev1ex1.jpg)
  cases.jsonl                   test cases, one JSON object per line
  reference/solution.py         reference solutions (optional, any languages), used to compute expected outputs
  reference/solution.go
  templates/solution.go         starter code (optional; generated from the signature if missing)
  templates/solution.py
```

### problem.yaml

| Field | Meaning |
|---|---|
| `id` | Problem number (a string, optional), used for sorting and lookup |
| `slug` | Unique identifier in lowercase kebab-case (required) |
| `title` | Title (required) |
| `difficulty` | `easy`, `medium` or `hard` |
| `tags` | List of tags |
| `kind` | `function` (one function) or `class` (a design problem) |
| `function` | Signature of a function problem: `name`, `params[{name,type}]`, `returns` |
| `class` | Signature of a design problem: `name`, `constructor[{name,type}]`, `methods[{name,params,returns}]` |
| `judge.compare` | How outputs are compared (see below). Default `exact` |
| `judge.tolerance` | Floating-point tolerance. Defaults to `1e-5` when returning `double`; either the absolute or the relative error must be within it |
| `judge.output` | For in-place problems: the function returns `void`, and the judge compares this parameter's value after the call |
| `judge.time_limit` | Time limit for a whole run, e.g. `5s`. Defaults to `timeout` in hc.yaml |
| `source` | Where the problem comes from: `site`, `url` |

Comparison modes:

| `compare` | Meaning | Examples |
|---|---|---|
| `exact` | The JSON values are equal (numbers compare by value) | Most problems |
| `unordered` | The outermost array is a multiset; order is ignored | Two Sum, Permutations |
| `unordered_deep` | Order is ignored at every array level | Group Anagrams, Subsets |
| `any_of` | `expected` is an array of all acceptable answers | Problems with a small set of valid answers |

### Types

Type names are LeetCode's `metaData` names, so problems fetched from LeetCode need no translation:

| Type | JSON | Go | Python |
|---|---|---|---|
| `integer` / `long` | number | `int` / `int64` | `int` |
| `double` | number | `float64` | `float` |
| `boolean` | `true` / `false` | `bool` | `bool` |
| `string` | string | `string` | `str` |
| `character` | one-character string `"a"` | `byte` | `str` |
| `ListNode` | array `[1,2,3]` | `*ListNode` | `Optional[ListNode]` |
| `TreeNode` | level-order array `[3,9,20,null,null,15,7]` | `*TreeNode` | `Optional[TreeNode]` |
| `T[]` | array, up to 3 levels; 1 level for node arrays | `[]T` | `List[T]` |
| `void` | return type only | — | `None` |

`list<T>` is normalized to `T[]`. Special types such as `Node` (graphs, N-ary trees, lists with random pointers) and `NestedInteger` are not supported yet.

### cases.jsonl

One case per line:

```json
{"input": {"nums": [2,7,11,15], "target": 9}, "expected": [0,1], "example": true}
{"input": {"nums": [-1,-2,-3,-4,-5], "target": -8}, "expected": [2,4], "note": "negative numbers"}
```

- `input`: for function problems, parameter name → value. For design problems, `{"ops": [...], "args": [[...], ...]}`; `ops[0]` must be the class name (the constructor).
- `expected`: the expected output. For design problems, an array with each operation's return value, using `null` for the constructor and `void` methods. If missing, `hc cases build` computes it with the reference solution.
- `example`: the case is an example from the statement. `hc test` runs only these by default.
- `note`: a description of the case.
- Blank lines and lines starting with `//` are ignored. Cases are numbered from 1, counting only the lines that hold a case.

### Validation

```
hc validate <slug>          # check against the protocol
hc validate <slug> --run    # also run the reference solution on every case; all must pass
```

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
