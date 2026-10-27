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
class Solution:
    def sumArray(self, nums: list[int]) -> int:
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
  checker.py                    the checker, with judge.compare: checker (see [Checkers](#checkers))
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
| `function` | Signature of a function problem: `name`, `params` (a list of `{name, type}`, each with at most one of `cycle`, `join`, `ref`, `copy`), `hidden` (inputs not passed to the solution), `returns` (defaults to `void`), `returns_ref`, `returns_copy`. See [structure inputs](#structure-inputs) and [`returns_copy`](#returns_copy) |
| `class` | Signature of a design problem: `name`, `constructor` (a list of `{name, type}`), `methods` (a list of `{name, params, returns}`) |
| `judge.compare` | How outputs are compared (see below). No default: a problem without it is *not ready* (see below) |
| `judge.tolerance` | Floating-point tolerance. Defaults to `1e-5` when returning `double`; either the absolute or the relative error must be within it |
| `judge.output` | For in-place problems: the judge compares this parameter's (or hidden input's) value after the call instead of the return value |
| `judge.output_length` | `return`: judge only the first k elements of `judge.output`, k being the (integer) return value. LeetCode's "remove duplicates" problems |
| `judge.round_trip` | For design problems that encode a value and decode it back: `encode`, `decode` (method names) and `instances` (`separate` or `same`). See [round-trip problems](#round-trip-problems) |
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
| `checker` | `checker.py` in the problem directory decides; `expected` is one valid answer | Many valid answers ("return any of them"), randomized output |

`unordered` and `unordered_deep` need an array result.

### Round-trip problems

Some design problems ask for an encoder and a decoder, in a format of the solution's choosing: Serialize and Deserialize Binary Tree (297), Serialize and Deserialize BST (449), Encode and Decode TinyURL (535). Only `decode(encode(x))` can be judged; the encoded string can neither be written in a case nor compared. `judge.round_trip` describes such a problem:

```yaml
kind: class
class:
  name: Codec
  constructor: []
  methods:
    - name: serialize
      params:
        - {name: root, type: TreeNode}
      returns: string
    - name: deserialize
      params:
        - {name: data, type: string}
      returns: TreeNode
judge:
  compare: exact
  round_trip:
    encode: serialize
    decode: deserialize
    instances: separate   # separate | same
```

A case gives the value to encode, keyed by the encoder's parameter name, instead of `ops` and `args`; `expected` is the decoder's result, which `hc cases build` fills from the reference as usual. For a correct codec it equals the input:

```json
{"input": {"root": [1,2,3,null,null,4,5]}, "expected": [1,2,3,null,null,4,5], "example": true}
```

For each case the driver decodes the input, constructs an instance and calls `encode` with it, then calls `decode` with the string `encode` returned, and judges `decode`'s result with `judge.compare` (`exact`, or a [checker](#checkers)). `instances` has no default, because it decides whether a solution may keep state between the two calls:

- `separate`: `decode` runs on a second instance, as in LeetCode's `ser = Codec(); deser = Codec(); ans = deser.deserialize(ser.serialize(root))` (297, 449). A codec that only remembers the tree on its instance fails.
- `same`: both calls run on one instance (535: `codec.decode(codec.encode(url))`), so a codec may keep a mapping.

Both instances live in the same process, as on LeetCode. An `encode` that does not return a string is a runtime error. The run time covers both calls. Other methods of the class are never called, and a requirement such as 449's "as compact as possible" is not judged.

Rules: `round_trip` is only for design problems; `encode` and `decode` name two different methods of the class; `encode` takes exactly one parameter and returns `string`; `decode` takes exactly one `string` parameter and returns the type of `encode`'s parameter; the constructor takes no parameters; `instances` is `separate` or `same`.

### Checkers

When a case has too many valid answers to list, or the output is random, a program decides: `judge.compare: checker` and a `checker.py` next to `problem.yaml`. Checkers are written in Python whatever language the solution is in, so problems judged by one need Python 3.10 or later even for Go solutions; the virtual environment hc manages (`.hc/venv`) provides it.

```python
# checker.py
def check(input, output, expected):
    """Return None or True to accept; False or a message string to reject."""
```

- `input` is the case's input as decoded JSON: the object of parameters (and hidden inputs) for function problems, `{"ops": [...], "args": [...]}` for design problems, the encoder's parameter for [round trips](#round-trip-problems).
- `output` is the solution's output as decoded JSON, in the same form as `expected` (the value of `judge.output` for in-place problems, one result per operation for design problems, the decoder's result for round trips).
- `expected` is the case's expected output, computed by the reference solution with `hc cases build` as for any other problem. It is one valid answer, so a checker can compare a derived value instead of solving the problem again: the length of the longest palindrome, the size of a subset. A case without `expected` is not judged ("No Expected Output").
- To rebuild a structure, import the codecs and types solutions use: `from hc_runtime import decode, TreeNode` and `decode("TreeNode", output)`.
- A rejected case is a Wrong Answer, and the message is shown with it in `hc test`, `hc submit`, the TUI and the submission history. Write messages that say what is wrong: `"[1,2] is not a permutation of nums"`, `"length 2, but the longest palindrome has length 3"`.

Longest Palindromic Substring (5), where `"bab"` and `"aba"` are both right:

```python
def check(input, output, expected):
    s = input["s"]
    if not isinstance(output, str) or output not in s:
        return "%r is not a substring of s" % (output,)
    if output != output[::-1]:
        return "%r is not a palindrome" % output
    if len(output) != len(expected):
        return "%r has length %d, but the longest palindrome has length %d" % (output, len(output), len(expected))
```

hc runs the checker once per run, after the solution: a helper calls `check` on every case that produced an output (cases that already failed with a runtime error or time limit are not sent). It gets the same time limit as the solution (`judge.time_limit`). A checker that cannot be loaded, raises, returns anything but `None`, a bool or a string, or runs out of time is a broken problem rather than a wrong answer: the run fails with an error naming `checker.py`, the case and the traceback, and `hc submit` records nothing. What the checker prints is shown only then.

**Randomized problems** (Shuffle an Array, Random Pick with Weight) are design problems whose cases make many calls, from hundreds to a few thousand. The checker checks that every output is valid and that their distribution is plausible: compare the observed frequencies with the expected ones with bounds generous enough that a correct solution essentially never fails (state the false-reject probability in the checker, e.g. below 1e-6 per case), while an obviously non-random solution (always the same pick, the identity shuffle) fails.

The built-in samples 5, 108, 210, 324 (in place), 384 and 528 (randomized) show checkers of each kind.

### Types

Type names are LeetCode's `metaData` names, so problems fetched from LeetCode need no translation:

| Type | JSON | Go | Python |
|---|---|---|---|
| `integer` / `long` | whole number | `int` / `int64` | `int` |
| `double` | number | `float64` | `float` |
| `boolean` | `true` / `false` | `bool` | `bool` |
| `string` | string | `string` | `str` |
| `character` | one-character string `"a"` | `byte` | `str` |
| `ListNode` | array `[1,2,3]` | `*ListNode` | `ListNode \| None` |
| `TreeNode` | level-order array `[3,9,20,null,null,15,7]` | `*TreeNode` | `TreeNode \| None` |
| `GraphNode`, `NaryNode`, `RandomListNode`, `NextTreeNode`, `QuadNode`, `MultilevelNode` | see [LeetCode's Node types](#leetcodes-node-types) | `*Node` | `Node \| None` |
| `NestedInteger` | integer or (nested) array, `[1,[4,[6]]]`; see [below](#nestedinteger-and-employee) | `*NestedInteger` | `NestedInteger` |
| `Employee` | `[id, importance, [subordinate ids]]` | `*Employee` | `Employee` |
| `T[]` | array, up to 3 levels; 1 level for `ListNode`, `TreeNode`, `NestedInteger` and `Employee`; none for the `Node` types | `[]T` | `list[T]` |
| `void` | return type only | — | `None` |

`list<T>` is normalized to `T[]`, and `int`, `bool`, `char` and `float` to `integer`, `boolean`, `character` and `double`.

### LeetCode's Node types

LeetCode calls six different structures `Node`. hc gives each its own type name; in solution code every one of them is still a type (Go) or class (Python) named `Node`, with LeetCode's field names, so a problem can use only one of them. They are not allowed in arrays, and the [structure inputs](#structure-inputs) are only for `ListNode` and `TreeNode`.

| Type | Fields (Go; Python uses the lower-case names) | JSON (case input and expected output) | Problems |
|---|---|---|---|
| `GraphNode` | `Val int`, `Neighbors []*Node` | Adjacency list: entry i lists the neighbors of node i+1, whose value is i+1. The value is node 1. `[[2,4],[1,3],[2,4],[1,3]]`; `[[]]` is a single node, `[]` no node | 133 |
| `NaryNode` | `Val int`, `Children []*Node` | Level order: the root, `null`, then the children of each node in level order, each group followed by `null` (trailing `null`s trimmed): `[1,null,3,2,4,null,5,6]` | 429, 559, 589, 590 |
| `RandomListNode` | `Val int`, `Next *Node`, `Random *Node` | `[[val, randomIndex], ...]`, `randomIndex` being the 0-based index of the random node or `null`: `[[7,null],[13,0],[11,4],[10,2],[1,0]]` | 138 |
| `NextTreeNode` | `Val int`, `Left *Node`, `Right *Node`, `Next *Node` | Input: level order like `TreeNode`, with every next pointer unset. Output: the levels read by following next pointers from each level's leftmost node, `[[1],[2,3],[4,5,7]]` (LeetCode writes `[1,#,2,3,#,4,5,7,#]`, which is not JSON) | 116, 117 |
| `QuadNode` | `Val bool`, `IsLeaf bool`, `TopLeft`, `TopRight`, `BottomLeft`, `BottomRight *Node` (Python: `val`, `isLeaf`, `topLeft`, ...) | Level order in which every node has four child slots (`null` when absent), each node written `[isLeaf, val]` with 0 or 1, trailing `null`s trimmed: `[[0,1],[1,0],[1,1],[1,1],[1,0]]` | 427, 558 |
| `MultilevelNode` | `Val int`, `Prev *Node`, `Next *Node`, `Child *Node` | LeetCode's serialization: the levels one after the other, each ended by `null`; a child level is preceded by as many more `null`s as its parent's 0-based index in the level above: `[1,2,3,4,5,6,null,null,null,7,8,9,10,null,null,11,12]` | 430 |

- In Go, the driver defines `Node` for problems that use one, so don't declare it. In Python, import it from `hc_runtime` under the name `Node`; starters do it for you: `from hc_runtime import GraphNode as Node`. Python constructors take LeetCode's arguments, all optional: `Node(val, neighbors)`, `Node(val, children)`, `Node(x, next, random)`, `Node(val, left, right, next)`, `Node(val, isLeaf, topLeft, topRight, bottomLeft, bottomRight)`, `Node(val, prev, next, child)`.
- A solution may build its own nodes (clones do). Returned structures are written in the same JSON form, and these are runtime errors: a `GraphNode` result that is not node 1 of a graph valued 1 to n; a random pointer to a node outside the returned list; a `MultilevelNode` whose `prev` does not point to the node before it (or is set on the first node of a level), and two nodes of one level with a child, which the serialization cannot express.
- A `QuadNode` that is not a leaf may have either value, so it is always written `[0,1]`.

### NestedInteger and Employee

Two more types are defined by the judge rather than by the solution, under LeetCode's names:

- `NestedInteger` holds an integer or a list of `NestedInteger`s. Its JSON form is an integer or an array of such values; `NestedInteger[]` is an array of them: `[[1,1],2,[1,1]]` (Flatten Nested List Iterator, 341). hc implements LeetCode's interface, so a solution can also build one to return (Mini Parser, 385). Go: `IsInteger() bool`, `GetInteger() int`, `SetInteger(int)`, `Add(NestedInteger)`, `GetList() []*NestedInteger`; the zero value is an empty list. Python: `NestedInteger(value=None)` (no value: an empty list), `isInteger()`, `getInteger()`, `setInteger(value)`, `add(elem)`, `getList()`.
- `Employee` has the fields `Id`, `Importance`, `Subordinates []int` (Python: `id`, `importance`, `subordinates`) and is written `[id, importance, [subordinate ids]]`. In an `Employee[]` the ids must be distinct and every subordinate must be one of them: `[[1,5,[2,3]],[2,3,[]],[3,3,[]]]` (Employee Importance, 690).

Go defines them in the driver of problems that use them. Python imports them from `hc_runtime`: `from hc_runtime import NestedInteger`.

#### `returns_copy`

`returns_copy: <param>` in `function` says the function returns a deep copy of a parameter (Clone Graph, Copy List with Random Pointer). If the returned structure shares any node with the parameter's structure, the case is a runtime error. The parameter must have the return type, which must be `ListNode`, `TreeNode` or one of the `Node` types; `returns_copy` cannot be combined with `returns_ref` or `judge.output`.

```yaml
function:
  name: cloneGraph
  params:
    - {name: node, type: GraphNode}
  returns: GraphNode
  returns_copy: node
judge:
  compare: exact
```

```json
{"input": {"node": [[2,4],[1,3],[2,4],[1,3]]}, "expected": [[2,4],[1,3],[2,4],[1,3]]}
```

### Structure inputs

Some problems pass the solution linked structures that a plain JSON value cannot describe: a list whose tail links back into it, two lists sharing their tail, a node inside a tree. Function problems describe them with these attributes, as LeetCode's judge does with its own test data:

| Attribute | Where | Meaning |
|---|---|---|
| `hidden` | `function` | Extra case inputs (`{name, type}`) the driver uses but does not pass to the solution |
| `cycle: <input>` | `ListNode` parameter | The list's tail links to the node at index `<input>` (an integer hidden input, 0-based); `-1` means no cycle |
| `join: {list, skip, at}` | `ListNode` parameter | The list keeps its first `skip` nodes, then continues with the nodes of the parameter `list` from index `at` (shared nodes, not copies) |
| `ref: <input>` | `ListNode` / `TreeNode` parameter | The case gives a node's **value**; the solution receives the node of the structure `<input>` holding it |
| `copy: <param>` | `ListNode` / `TreeNode` parameter | Not in the case; the solution receives a deep copy of `<param>` |
| `returns_ref: <input>` | `function` | The function returns a node of the structure `<input>` (or none), judged by identity |

A parameter has at most one of `cycle`, `join`, `ref` and `copy`. Starters show only the parameters, with their node types (`head *ListNode`, `p: TreeNode | None`).

**`cycle`**, Linked List Cycle II. The case gives `head` and `pos`; the solution receives only `head`:

```yaml
function:
  name: detectCycle
  params:
    - {name: head, type: ListNode, cycle: pos}
  hidden:
    - {name: pos, type: integer}
  returns: ListNode
  returns_ref: head
```

```json
{"input": {"head": [3,2,0,-4], "pos": 1}, "expected": 1}
```

**`join`**, Intersection of Two Linked Lists. `headA` and `headB` are the full lists, including the shared tail; `skipA` and `skipB` count the nodes before it. When `skipA` is the length of `headA` the lists do not intersect, and `skipB` is the length of `headB`:

```yaml
function:
  name: getIntersectionNode
  params:
    - {name: headA, type: ListNode}
    - {name: headB, type: ListNode, join: {list: headA, skip: skipB, at: skipA}}
  hidden:
    - {name: skipA, type: integer}
    - {name: skipB, type: integer}
  returns: ListNode
  returns_ref: headA
```

```json
{"input": {"headA": [4,1,8,4,5], "headB": [5,6,1,8,4,5], "skipA": 2, "skipB": 3}, "expected": 2}
{"input": {"headA": [2,6,4], "headB": [1,5], "skipA": 3, "skipB": 2}, "expected": null}
```

**`ref`**, Lowest Common Ancestor of a Binary Tree. `p` and `q` are values in the case and nodes of `root` in the solution. A referenced value must occur exactly once in its structure:

```yaml
function:
  name: lowestCommonAncestor
  params:
    - {name: root, type: TreeNode}
    - {name: p, type: TreeNode, ref: root}
    - {name: q, type: TreeNode, ref: root}
  returns: TreeNode
  returns_ref: root
```

```json
{"input": {"root": [3,5,1,6,2,0,8,null,null,7,4], "p": 5, "q": 1}, "expected": 3}
```

A `ref` may point into a hidden input, and `judge.output` may name one. Delete Node in a Linked List gives the solution only `node`, and judges the whole list:

```yaml
function:
  name: deleteNode
  params:
    - {name: node, type: ListNode, ref: head}
  hidden:
    - {name: head, type: ListNode}
  returns: void
judge:
  compare: exact
  output: head
```

```json
{"input": {"node": 5, "head": [4,5,1,9]}, "expected": [4,1,9]}
```

**`copy`**, Find a Corresponding Node of a Binary Tree in a Clone of That Tree. `cloned` is not in the case:

```yaml
function:
  name: getTargetCopy
  params:
    - {name: original, type: TreeNode}
    - {name: cloned, type: TreeNode, copy: original}
    - {name: target, type: TreeNode, ref: original}
  returns: TreeNode
  returns_ref: cloned
```

```json
{"input": {"original": [7,4,3,null,null,6,19], "target": 3}, "expected": 3}
```

**`returns_ref`** encodes the returned node so that it is judged by identity, not by the values after it: in a `ListNode` structure as its 0-based **index** (list values may repeat), in a `TreeNode` structure as its **value** (problems referring to tree nodes have unique values), and `null` for no node. A returned node that is not part of the structure is a runtime error. `expected` uses the same encoding.

Rules: `cycle`, `skip` and `at` name integer hidden inputs; `join.list` names another plain `ListNode` parameter; `ref` and `returns_ref` name a parameter or hidden input of the same node type that holds a whole structure (not a `ref`); `copy` names a plain parameter of the same type; every hidden input must be used; `judge.output` cannot name a list with a cycle. None of this is available in design problems.

### cases.jsonl

One case per line:

```json
{"input": {"nums": [2,7,11,15], "target": 9}, "expected": [0,1], "example": true}
{"input": {"nums": [-1,-2,-3,-4,-5], "target": -8}, "expected": [2,4], "note": "negative numbers"}
```

- `input`: for function problems, an object with exactly one value per parameter and hidden input, keyed by name. A `ref` parameter takes the referenced node's value; a `copy` parameter is left out. For design problems, `{"ops": [...], "args": [[...], ...]}`; `ops[0]` must be the class name (the constructor), and each `args` entry holds exactly the arguments of its op. For [round-trip problems](#round-trip-problems), the encoder's parameter: `{"root": [1,2,3]}`.
- `expected`: the expected output. For design problems, an array with each operation's return value, using `null` for the constructor and `void` methods; for round trips, the decoder's result. For `any_of`, an array of acceptable answers, written by hand: a reference solution finds only one answer, so `hc cases build` refuses to fill it. For `checker`, one valid answer, passed to the checker. Otherwise, if missing, `hc cases build` computes it with the reference solution.
- `example`: the case is an example from the statement. `hc test` runs only these by default (the first 3 cases if none is marked).
- `note`: a description of the case.
- No other fields are allowed. Blank lines and lines starting with `//` are ignored. Cases are numbered from 1, counting only the lines that hold a case.

### Reference solutions and templates

A reference solution is written exactly like a user's solution, in `reference/solution.py` or `reference/solution.go`. `hc start` shows the expected shape: the generated starter is a reference with the body missing.

| | Function problem | Class problem |
|---|---|---|
| Python | `class Solution` with a method named `function.name`, or a module-level function of that name | A class named `class.name`; `__init__` takes the constructor parameters; methods have the names in `methods` |
| Go | `package main`; `func <function.name>(...)` with the Go types from the table above | `type <Name> struct`, `func Constructor(...) <Name>`, and methods with pointer receivers whose names start with an upper-case letter (`getMin` → `GetMin`), except the encoder and decoder of a round trip, which keep their names as in LeetCode's templates (`serialize`) |

- `ListNode` and `TreeNode` are provided: in Python, `from hc_runtime import ListNode, TreeNode`; in Go they are predefined (`Val`, `Next`, `Left`, `Right`), so don't declare them. So are the problem's [`Node`](#leetcodes-node-types), `NestedInteger` and `Employee`: `from hc_runtime import GraphNode as Node` or `from hc_runtime import NestedInteger` in Python, predefined in Go.
- With both references present, `hc cases build` uses Go, then Python; pick one with `-l python`. `hc validate --run` runs every reference.
- `templates/solution.go` and `templates/solution.py` replace the generated starter code. They must keep the signature, because the driver calls it.

### Language versions

| Language | Oldest supported version |
|---|---|
| Python | 3.10 |
| Go | 1.22 |

These versions bind the code hc generates (drivers, starters) and the code a problem ships: reference solutions and templates must work on them. Your own solutions may use anything your toolchain supports.

- `hc test`, `hc submit` and `hc debug` name the toolchain they run with (`[Python 3.12.9]`) and warn when it is older, but still run.
- `hc cases build` and `hc validate --run` refuse to run references on an older toolchain, so expected outputs always come from a supported one.
- Checkers always run on Python 3.10 or later, also for Go solutions.
- Python 3.10 is needed for the `X | None` annotations in LeetCode's templates. hc runs Python in a virtual environment it creates in `.hc/venv` with Python 3.10 or later (with uv, which downloads one if needed), so an older system `python3` (macOS ships 3.9) does not matter. `python.interpreter` in `hc.yaml` replaces the environment with an interpreter of your choice.
- Go 1.22 limits both the language and the standard library. To check Go code against it, put `go 1.22` in its `go.mod` and run `go vet`, whose `stdversion` check reports newer standard library symbols.

### Validation

```
hc validate <slug>          # check against the protocol
hc validate <slug> --run    # also run every reference solution on every case; all must pass
```

`hc validate` reports, with the case number and field:

- `problem.yaml`: syntax errors (with line and column), unknown fields, invalid slug, missing title, unsupported types, a `judge.output` that is not a parameter or hidden input, a `void` function without `judge.output`, misused structure inputs (`hidden`, `cycle`, `join`, `ref`, `copy`, `returns_ref`), a misused `returns_copy`, more than one `Node` type, a misused `judge.output_length`, a misused `judge.round_trip`, unknown `judge.compare`, `judge.compare: checker` without `checker.py` and `checker.py` with another mode, an invalid `judge.time_limit`;
- `cases.jsonl`: invalid JSON, unknown fields, missing or unknown inputs, values that don't match their declared type (`input "nums": [1]: must be integer, got string "x"`) or the form of a judge-defined type (a neighbor that is not a node, a random index outside the list, a misplaced `null`, an unknown subordinate), structure inputs that don't fit (a cycle position outside the list, joined lists whose tails differ, a referenced value that is missing or repeated), wrong argument counts and unknown methods in design problems, `ops` and `args` in a round trip, and an `expected` of the wrong shape;
- with `--run`: reference solutions that don't compile, crash, or disagree with `expected` (both values are shown, and the checker's message); with `checker`, every reference output goes through the checker, and a broken checker is an error.

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
    Toolchain() (lang.Toolchain, error) // detects the compiler or interpreter and its version; fails if missing
    Setup(ctx context.Context, w io.Writer) error // prepares an environment hc manages, e.g. Python's .hc/venv; called before running
}
```

A driver only **decodes the input, calls the user's code and encodes the output**. It never judges. All judging happens in Go (`internal/judge`), so a new language doesn't reimplement comparison. Decoding includes building [structure inputs](#structure-inputs) and [`Node` structures](#leetcodes-node-types), and encoding includes `returns_ref` and the `returns_copy` check; an input that does not fit its structure is reported as an error starting with `invalid input:`.

- `HC_CASES` names a JSON file containing `[{"id": 1, "input": {...}}, ...]`.
- `HC_OUT` names a file the driver appends results to, as JSON Lines, one per case:

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  A non-empty `error` means a runtime error; keep only the stack frames from the user's code if you can. `elapsed_ns` covers only the time spent in the user's code.
- The user's stdout and stderr go straight to the terminal, so print debugging works.
- If the driver crashes or times out, cases without a result line are judged Runtime Error or Time Limit Exceeded respectively.
- [Checkers](#checkers) are not the driver's business either: Python runs them for every language (`lang.Checker`), with `internal/judge` turning their results into verdicts.

The existing drivers are good references: `internal/lang/golang` (generated code plus a static runtime) and `internal/lang/python` (a dynamic runtime with the signature embedded).
