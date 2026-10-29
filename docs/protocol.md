# The handcoding problem protocol

English · [简体中文](protocol_zh.md)

## Scope and versions

This document specifies the handcoding problem protocol: how problems are collected in packs, how a problem is laid out on disk, what its files mean, how its cases are judged, and what the code it ships (reference solutions, templates, checkers) may assume. It is normative. [`docs/protocol_zh.md`](protocol_zh.md) is a translation; where the two differ, this English text applies.

MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. A problem that breaks a MUST is invalid; the [validity rules](#validity-rules) collect every such rule.

This is **protocol 1**. Every field, type and mode below is part of protocol 1 unless it says otherwise. A pack declares the protocol it is written against with `protocol: N` in its manifest (see [pack.yaml](#packyaml)); a newer protocol may add features, which an older pack does not use.

How hc itself behaves (which cases `hc test` runs, how `hc fetch` imports a problem, where a workspace keeps its packs) is documented in `docs/how-to-use.md`, not here. How hc talks to its language drivers is internal to hc, and documented in `CONTRIBUTING.md`.

### Protocol versions

Protocol versions are integers, and are not tied to hc's release versions: an hc release that does not change the protocol does not change what a pack may contain.

- A version that only adds features (a type, a compare mode, a field) keeps every pack written against an earlier version valid, with the same meaning. The spec marks each feature with the version that introduced it (the Since columns).
- A version that changes or removes a feature is incompatible: from that version on, tools stop reading packs that declare an earlier one, and those packs have to be updated.
- A pack MUST NOT use a feature introduced after the version it declares, so that every tool reading that version can read the pack.
- A tool reads the versions from the oldest it still supports to the newest it implements. It MUST refuse a pack outside that range, before reading anything else of it, and say what to do: for a newer pack, upgrade the tool; for an older one, update the pack. hc says "this pack needs protocol 2, hc v0.1.0 supports up to 1, upgrade hc", or "this pack uses protocol 1, which hc v0.5.0 no longer reads, update the pack".

| Version | Changes |
|---|---|
| 1 | The initial frozen protocol: packs and `pack.yaml`, problem directories, `problem.yaml`, types, signatures, cases, judging, checkers and the solution ABI, with Go and Python as the solution languages. |

### Out of scope

Protocol 1 does not define the following. Each would be a later, additive change:

- interactive problems, whose solution talks to a judge (issue #7);
- `tests` problems, judged by test code instead of cases (issue #31);
- case generators, and judging performance (issue #33);
- solution languages other than Go and Python.

## Packs

A pack is a set of problems published together, usually as a repository of its own. It is the unit that is installed and updated, and the scope of problem [identity](#identity). A pack is a directory holding:

```
pack.yaml          the manifest; see pack.yaml
problems/<dir>/    one problem directory per problem; see Problem directory
...                anything else is not part of the pack
```

- `pack.yaml` MUST be at the root of the pack.
- Every directory directly under `problems/` that holds a `problem.yaml` is a problem of the pack; its name MUST be the problem's canonical name ([Identity](#identity)). Other entries of `problems/` are not part of the pack. A pack without `problems/`, or with an empty one, has no problems, and is valid.
- Anything else in the pack (a README, a license, CI configuration, agent instructions and skills) is not part of the pack, and tools ignore it. Problems sit under `problems/` so that these files never look like problems.

### pack.yaml

`pack.yaml` is a YAML mapping. Unknown fields are errors, at every level.

| Field | Type | Required | Default | Constraints | Since |
|---|---|---|---|---|---|
| `protocol` | integer | yes | | The [protocol version](#protocol-versions) the pack is written against: a positive integer, not a string or a fraction | 1 |
| `name` | string | yes | | Lower-case letters, digits and single hyphens: `^[a-z0-9]+(-[a-z0-9]+)*$`. Not `local`. The pack's identity (below) | 1 |
| `title` | string | no | `name` | The display name | 1 |
| `version` | string | no | none | A SemVer 2.0.0 version without a `v` prefix: `0.3.0`, `1.0.0-alpha.1`. Informational: installs are pinned by commit, not by this field | 1 |
| `description` | string | no | none | Informational | 1 |
| `authors` | list of strings | no | `[]` | Informational | 1 |
| `license` | string | no | none | SHOULD be an SPDX license expression, such as `MIT` | 1 |
| `homepage` | string | no | none | SHOULD be a URL | 1 |
| `languages` | list of strings | no | every language of `protocol` | Not empty; language ids of the declared protocol, without duplicates; see below | 1 |
| `groups` | list of groups | no | `[]` | See [Groups](#groups) | 1 |
| `groups[].name` | string | yes | | `^[a-z0-9]+(-[a-z0-9]+)*$`, unique among the pack's groups | 1 |
| `groups[].title` | string | no | the group's `name` | The display name | 1 |
| `groups[].problems` | list of strings | no | `[]` | Slugs of problems of the pack, without duplicates | 1 |

- `protocol` is read first. A tool that does not read the declared version refuses the pack without reading the rest of it ([Protocol versions](#protocol-versions)), since a newer protocol may add fields it does not know.
- `name` is the pack's identity. Installed packs, solutions and submission history are keyed by it, so renaming a pack is a breaking change, as renaming a slug is for a problem. `local` is reserved for the problems a workspace holds outside any pack.
- `languages` lists the languages the pack's problems can be solved in. Solutions in other languages are not supported, and tools MAY refuse to start one. Leaving it out means every language of the declared protocol, so a pack gets the languages a later protocol adds only when it declares that protocol. References may be written in any language of the protocol, whatever `languages` says, since they only compute expected outputs; checkers are Python in any case ([Checkers](#checkers)).

The language ids:

| Id | Language | Since |
|---|---|---|
| `go` | Go | 1 |
| `python` | Python | 1 |

A pack.yaml with every field:

```yaml
protocol: 1
name: leetcode
title: LeetCode
version: 0.3.0
description: Classic LeetCode problems with Go and Python references.
authors: [MorePeanuts]
license: MIT
homepage: https://github.com/MorePeanuts/handcoding-leetcode
languages: [go, python]
groups:
  - name: arrays
    title: Arrays
    problems: [two-sum, rotate-array]
```

### Groups

Groups arrange the pack's problems into named lists, such as topics or study plans.

- `groups` is ordered, and so is each group's `problems`: tools show them in this order.
- A group refers to problems by `slug`, not by id or directory name, and every entry MUST be the slug of a problem of the pack. A group MUST NOT list a problem twice.
- A problem MAY be in several groups or in none. A group MAY be empty.

## Problem directory

A problem is a directory holding these entries:

| Entry | Required | Content |
|---|---|---|
| `problem.yaml` | yes | Metadata, signature and judge options; see [problem.yaml](#problemyaml) |
| `description.md` | no | The statement, in Markdown |
| `assets/` | no | Files the statement refers to, such as images |
| `cases.jsonl` | no | The test cases; see [Cases](#cases) |
| `reference/solution.py`, `reference/solution.go` | no | Reference solutions, any subset of the languages |
| `templates/solution.py`, `templates/solution.go` | no | Starter code, replacing the one generated from the signature |
| `checker.py` | with `judge.compare: checker` only | The checker; see [Checkers](#checkers) |

- A problem without `description.md` has an empty statement, and one without `cases.jsonl` has no cases. Both are valid, but such a problem cannot be practised meaningfully.
- The statement refers to local files by paths relative to the problem directory, and those files MUST be under `assets/`: `![](assets/rev1ex1.jpg)`.
- Other entries are not part of the problem. Tools MAY ignore or drop them.
- What reference solutions, templates and checkers may contain is specified in [Solution ABI](#solution-abi) and [Checkers](#checkers).

### Identity

- `slug` is the problem's identity within its pack. It MUST be unique in the pack; problems of different packs MAY share a slug. Solutions and submission history are keyed by the pack's name and the slug (through the directory name), so renaming a slug is a breaking change for the pack.
- `id` is optional. When present it MUST be unique in the pack. It is used for sorting and lookup.
- The directory MUST have the canonical name: when `id` is a decimal integer n, `<n padded to 4 digits>-<slug>` (`id: "1"`, slug `two-sum`: `0001-two-sum`; `id: "12345"`: `12345-<slug>`); otherwise `<slug>`.

## problem.yaml

`problem.yaml` is a YAML mapping. Unknown fields are errors, at every level.

| Field | Type | Required | Default | Constraints | Since |
|---|---|---|---|---|---|
| `id` | string | no | none | Unique in the pack; see [Identity](#identity). A YAML integer is read as its decimal text | 1 |
| `slug` | string | yes | | Letters, digits and single hyphens: `^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`. SHOULD be lower case. Unique in the pack | 1 |
| `title` | string | yes | | Not empty | 1 |
| `difficulty` | string | no | none | `easy`, `medium` or `hard`, in any case | 1 |
| `tags` | list of strings | no | `[]` | | 1 |
| `kind` | string | yes | | `function` or `class`. Never inferred from the sections present | 1 |
| `function` | mapping | with `kind: function` | | See [Function problems](#function-problems) | 1 |
| `class` | mapping | with `kind: class` | | See [Class problems](#class-problems) | 1 |
| `judge.compare` | string | no | none | `exact`, `unordered`, `unordered_deep`, `any_of` or `checker`; see [Compare modes](#compare-modes). Without it the problem is not [ready](#readiness) | 1 |
| `judge.tolerance` | number | no | `1e-5` if the judged value (in a class problem that is not a round trip, a method's return value) has base type `double`; `0` otherwise | A finite number, 0 or more. `0` compares numbers exactly. See [Compare modes](#compare-modes) | 1 |
| `judge.output` | string | for a `void` function | none | Function problems only: a parameter or hidden input; see [In-place output](#in-place-output) | 1 |
| `judge.output_length` | string | no | none | Function problems only: `return`; see [In-place output](#in-place-output) | 1 |
| `judge.round_trip` | mapping | no | none | Class problems only: `encode`, `decode`, `instances`; see [Round-trip problems](#round-trip-problems) | 1 |
| `judge.timeout` | string | no | the workspace's `timeout` (10s unless configured) | A positive duration such as `5s` or `500ms`; see [Timeout](#timeout) | 1 |
| `source.site` | string | no | none | Free text, e.g. `leetcode.com` | 1 |
| `source.url` | string | no | none | | 1 |

The section that does not match `kind` (`class` in a function problem, `function` in a class problem) MUST NOT be present.

> **YAML pitfall.** Inside YAML's flow syntax (`{...}`, `[...]`), `[]` is YAML syntax, so an array type there MUST be quoted: `{name: nums, type: "integer[]"}`. In block style (`type: integer[]` on its own line) quotes are optional.

Problem 0001:

```yaml
id: "1"
slug: two-sum
title: Two Sum
difficulty: easy
tags: [array, hash-table]
kind: function
function:
  name: twoSum
  params:
    - {name: nums, type: "integer[]"}
    - {name: target, type: integer}
  returns: "integer[]"
judge:
  compare: unordered
```

## Types

Type names are LeetCode's `metaData` names. A type is a base type followed by zero or more `[]`. These aliases are accepted and normalized: `list<T>` to `T[]` (nested: `list<list<integer>>` is `integer[][]`), `int` to `integer`, `bool` to `boolean`, `char` to `character`, `float` to `double`.

Every value in a case, an expected output and a solution's output is JSON. This section defines, for each type, its JSON encoding and its binding in each solution language. The encodings are the same for inputs (`input` in a case) and outputs (`expected`, and what a solution's result is encoded to), except where a type says otherwise.

| Type | JSON | Go | Python | Since |
|---|---|---|---|---|
| `integer` | integer | `int` | `int` | 1 |
| `long` | integer | `int64` | `int` | 1 |
| `double` | number | `float64` | `float` | 1 |
| `boolean` | `true` or `false` | `bool` | `bool` | 1 |
| `string` | string | `string` | `str` | 1 |
| `character` | string of one byte | `byte` | `str` | 1 |
| `ListNode` | see [ListNode and TreeNode](#listnode-and-treenode) | `*ListNode` | `ListNode \| None` | 1 |
| `TreeNode` | see [ListNode and TreeNode](#listnode-and-treenode) | `*TreeNode` | `TreeNode \| None` | 1 |
| `GraphNode`, `NaryNode`, `RandomListNode`, `NextTreeNode`, `QuadNode`, `MultilevelNode` | see [LeetCode's Node types](#leetcodes-node-types) | `*Node` | `Node \| None` | 1 |
| `NestedInteger` | see [NestedInteger and Employee](#nestedinteger-and-employee) | `*NestedInteger` | `NestedInteger` | 1 |
| `Employee` | see [NestedInteger and Employee](#nestedinteger-and-employee) | `*Employee` | `Employee` | 1 |
| `T[]` | see [Arrays](#arrays) | `[]T` | `list[T]` | 1 |
| `void` | none; return types only | no result | `None` | 1 |

### Scalars

- `integer` and `long`: a JSON number written without fraction or exponent (`3`, not `3.0` or `3e0`), in the signed 64-bit range. The protocol does not limit `integer` to 32 bits.
- `double`: any JSON number. Integers are valid doubles (`2`, `2.0`).
- `boolean`: `true` or `false`.
- `string`: a JSON string, holding any Unicode text.
- `character`: a JSON string of exactly one byte, so an ASCII character: `"a"`.

### Arrays

- `T[]` is a JSON array whose elements are values of `T`.
- Arrays nest to at most 3 levels (`integer[][][]`). Arrays of `ListNode`, `TreeNode`, `NestedInteger` and `Employee` have 1 level at most. The `Node` types are never in arrays.
- A solution's array result that is Go's `nil` or Python's `None` is encoded as `[]`.

### ListNode and TreeNode

Go: `type ListNode struct { Val int; Next *ListNode }` and `type TreeNode struct { Val int; Left, Right *TreeNode }`. Python: `ListNode(val=0, next=None)` and `TreeNode(val=0, left=None, right=None)`, with attributes `val`, `next`, `left`, `right`. Node values are integers.

- `ListNode`: the array of the values from the head on, `[1,2,3]`. `[]` is the empty list. In inputs, `null` also means the empty list.
- `TreeNode`: the level-order array, `null` for a missing child: `[3,9,20,null,null,15,7]`. The first element is the root's value, and each next pair of elements gives the children of the next node (not `null`) in level order. Outputs have trailing `null`s removed. `[]` is the empty tree; in inputs, `null` and `[null]` also mean the empty tree. An array listing children for more nodes than it has is invalid, such as `[1,null,null,2]` or `[null,1]`.

### LeetCode's Node types

LeetCode calls six different structures `Node`. The protocol gives each its own type name. In solution code each is a type (Go) or class (Python) named `Node` with LeetCode's field names, so a problem MUST NOT use more than one of them.

| Type | Fields (Go; Python uses the same names starting with a lower-case letter) | JSON | LeetCode problems |
|---|---|---|---|
| `GraphNode` | `Val int`, `Neighbors []*Node` | Adjacency list of a graph whose nodes have the values 1 to n: entry i (0-based) lists, by value, the neighbors of the node with value i+1, and the value of the type is the node with value 1. `[[2,4],[1,3],[2,4],[1,3]]`; `[[]]` is a single node, `[]` no node. Every neighbor MUST be between 1 and n | 133 |
| `NaryNode` | `Val int`, `Children []*Node` | Level order: the root's value, `null`, then the children of each node in level order, each group followed by `null`, trailing `null`s removed: `[1,null,3,2,4,null,5,6]`. `[]` is no tree. There MUST NOT be more groups than nodes | 429, 559, 589, 590 |
| `RandomListNode` | `Val int`, `Next *Node`, `Random *Node` | `[[val, randomIndex], ...]` in list order, `randomIndex` being the 0-based index of the node `Random` points to, or `null`: `[[7,null],[13,0],[11,4],[10,2],[1,0]]`. Every index MUST be in the list | 138 |
| `NextTreeNode` | `Val int`, `Left *Node`, `Right *Node`, `Next *Node` | Input: the level order of `TreeNode`, all `Next` pointers unset. Output (and `expected`): an `integer[][]`, the values of each level read by following `Next` from its leftmost node: `[[1],[2,3],[4,5,7]]` | 116, 117 |
| `QuadNode` | `Val bool`, `IsLeaf bool`, `TopLeft`, `TopRight`, `BottomLeft`, `BottomRight *Node` | Level order in which every node has four child slots, `null` when empty, each node written `[isLeaf, val]` with `0` or `1`, trailing `null`s removed: `[[0,1],[1,0],[1,1],[1,1],[1,0]]`. A node that is not a leaf is output as `[0,1]`, whatever its value | 427, 558 |
| `MultilevelNode` | `Val int`, `Prev *Node`, `Next *Node`, `Child *Node` | The levels one after the other, each ended by `null`; a child level is preceded by as many more `null`s as its parent's 0-based index in the level above: `[1,2,3,4,5,6,null,null,null,7,8,9,10,null,null,11,12]` | 430 |

- In inputs, `null` means no node for every `Node` type; outputs write no node as `[]`. A `MultilevelNode` output ends with its last value.
- Python constructors take LeetCode's arguments, all optional: `Node(val, neighbors)`, `Node(val, children)`, `Node(x, next, random)` (stored as `val`), `Node(val, left, right, next)`, `Node(val, isLeaf, topLeft, topRight, bottomLeft, bottomRight)`, `Node(val, prev, next, child)`.
- A solution MAY return nodes it built itself. A returned structure that the encoding cannot express is a Runtime Error: a `GraphNode` that is not node 1 of a graph whose values are 1 to n, each once; a random pointer to a node outside the returned list; a `MultilevelNode` whose `Prev` does not point to the node before it, or is set on the first node of a level; two nodes of one level with a child.

### NestedInteger and Employee

- `NestedInteger` holds an integer or a list of `NestedInteger`s. JSON: an integer, or an array of `NestedInteger` values: `[1,[4,[6]]]`. `NestedInteger[]` is an array of them (problem 0341: `[[1,1],2,[1,1]]`). The interface is LeetCode's, so a solution can also build one (LeetCode 385):
  - Go: `IsInteger() bool`, `GetInteger() int`, `SetInteger(int)`, `Add(NestedInteger)`, `GetList() []*NestedInteger`. The zero value is an empty list.
  - Python: `NestedInteger(value=None)` (without a value, an empty list), `isInteger()`, `getInteger()`, `setInteger(value)`, `add(elem)`, `getList()`.
- `Employee` has the fields `Id int`, `Importance int`, `Subordinates []int` (Python: `Employee(id, importance, subordinates)`). JSON: `[id, importance, [subordinate ids]]`. In an `Employee[]` the ids MUST be distinct and every subordinate MUST be one of them: `[[1,5,[2,3]],[2,3,[]],[3,3,[]]]` (LeetCode 690).

## Signatures

`kind: function` problems ask for one function; `kind: class` problems ask for a class driven by a sequence of operations (LeetCode's design problems). A parameter is a mapping `{name, type}`; names MUST be non-empty and unique within their list (in a function, across `params` and `hidden` together), and parameter types MUST NOT be `void`.

### Function problems

| Field | Type | Required | Default | Constraints | Since |
|---|---|---|---|---|---|
| `function.name` | string | yes | | The name of the function the solution defines | 1 |
| `function.params` | list of parameters | no | `[]` | Passed to the solution in this order. Each MAY have one of `cycle`, `join`, `ref`, `copy`; see [Structure inputs](#structure-inputs) | 1 |
| `function.hidden` | list of parameters | no | `[]` | Case inputs not passed to the solution; see [Structure inputs](#structure-inputs). Names MUST differ from the parameters' | 1 |
| `function.returns` | type | no | `void` | | 1 |
| `function.returns_ref` | string | no | none | See [Structure inputs](#structure-inputs) | 1 |
| `function.returns_copy` | string | no | none | See [returns_copy](#returns_copy) | 1 |

The judged value is the return value, or the value of `judge.output` after the call ([In-place output](#in-place-output)). A `void` function MUST have `judge.output`.

### Class problems

| Field | Type | Required | Default | Constraints | Since |
|---|---|---|---|---|---|
| `class.name` | string | yes | | The name of the class the solution defines | 1 |
| `class.constructor` | list of parameters | no | `[]` | The constructor's parameters | 1 |
| `class.methods` | list of methods | yes | | At least one | 1 |
| `class.methods[].name` | string | yes | | Not empty, and unique among the methods, also when their first letters are capitalized (`get` and `Get` are one method in Go) | 1 |
| `class.methods[].params` | list of parameters | no | `[]` | | 1 |
| `class.methods[].returns` | type | no | `void` | | 1 |

A case gives a sequence of operations: the constructor, then method calls on the instance it made. The judged value is the array of their results; see [Cases](#cases). Class problems have no [structure inputs](#structure-inputs), and MUST NOT have `judge.output` or `judge.output_length`.

Problem 0146:

```yaml
kind: class
class:
  name: LRUCache
  constructor:
    - {name: capacity, type: integer}
  methods:
    - name: get
      params:
        - {name: key, type: integer}
      returns: integer
    - name: put
      params:
        - {name: key, type: integer}
        - {name: value, type: integer}
      returns: void
judge:
  compare: exact
```

### Structure inputs

Some function problems pass the solution linked structures that one JSON value cannot describe: a list whose tail links back into it, two lists sharing their tail, a node inside a tree. These attributes describe them:

| Attribute | On | Meaning | Since |
|---|---|---|---|
| `hidden` | `function` | Case inputs (`{name, type}`) used to build parameters or to judge, and not passed to the solution | 1 |
| `cycle: <input>` | a `ListNode` parameter | The list's last node links to the node at index `<input>`, an `integer` hidden input (0-based); `-1` means no cycle | 1 |
| `join: {list, skip, at}` | a `ListNode` parameter | The list keeps its first `skip` nodes, then continues with the nodes of the parameter `list` from index `at` on: the same nodes, not copies. `skip` and `at` name `integer` hidden inputs | 1 |
| `ref: <input>` | a `ListNode` or `TreeNode` parameter | The case gives a node's value; the solution receives the node of the structure `<input>` that holds it | 1 |
| `copy: <param>` | a `ListNode` or `TreeNode` parameter | Not given in the case; the solution receives a deep copy of the parameter `<param>` | 1 |
| `returns_ref: <input>` | `function` | The function returns a node of the structure `<input>`, or none, judged by identity | 1 |

Rules:

- A parameter has at most one of `cycle`, `join`, `ref` and `copy`. Hidden inputs have none of them.
- `cycle`, `join.skip` and `join.at` name hidden inputs of type `integer`.
- `join.list` names another parameter of type `ListNode` that has none of the four attributes.
- `ref` names another parameter or hidden input of the same type that is not itself a `ref`.
- `copy` names another parameter (not a hidden input) of the same type that has none of the four attributes.
- `returns_ref` requires a `ListNode` or `TreeNode` return type, and names a parameter or hidden input of that type that is not a `ref`. It excludes `judge.output`.
- Every hidden input is used by a `cycle`, `join`, `ref`, `returns_ref` or `judge.output`.
- `judge.output` MUST NOT name a parameter with a `cycle`.

In a case:

- A `cycle` input is between `-1` and the list's length minus 1.
- For a `join`, `skip` is between 0 and the joined parameter's length, `at` between 0 and the length of `list`, and the joined parameter's values from index `skip` on are the values of `list` from index `at` on. `at` equal to the length of `list` (and so `skip` equal to the parameter's length) means the lists do not intersect.
- A `ref` parameter's value is an integer that occurs exactly once among the values of the structure it refers to (for a structure that is a `copy`, the values of its source).
- A `copy` parameter is absent.

`returns_ref` encodes the returned node, in outputs and in `expected`, by identity: in a `ListNode` structure as its 0-based index in node order, in a `TreeNode` structure as its value, and no node as `null`. Returning a node that is not part of the structure is a Runtime Error.

Problem 0142 (Linked List Cycle II) gives `head` and `pos`; the solution receives only `head`:

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

Problem 0160 (Intersection of Two Linked Lists): `headA` and `headB` are the full lists, including the shared tail:

```yaml
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
```

Problem 0236 (Lowest Common Ancestor of a Binary Tree): `p` and `q` are values in the case and nodes of `root` in the solution:

```yaml
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

Problem 0237 (Delete Node in a Linked List) refers into a hidden input and judges it:

```yaml
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
{"input": {"head": [4,5,1,9], "node": 5}, "expected": [4,1,9]}
```

Problem 1379 gives no `cloned` in the case:

```yaml
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

### returns_copy

`function.returns_copy: <param>` says that the function returns a deep copy of the parameter `<param>` (LeetCode's Clone Graph, Copy List with Random Pointer). If the returned structure shares a node with the parameter's structure, the case is a Runtime Error.

- `<param>` names a parameter (not a hidden input) whose type is the return type.
- The return type is `ListNode`, `TreeNode` or one of the `Node` types.
- `returns_copy` excludes `returns_ref` and `judge.output`.

Problem 0133:

```yaml
function:
  name: cloneGraph
  params:
    - {name: node, type: GraphNode}
  returns: GraphNode
  returns_copy: node
```

```json
{"input": {"node": [[2,4],[1,3],[2,4],[1,3]]}, "expected": [[2,4],[1,3],[2,4],[1,3]]}
```

### Round-trip problems

A class problem whose methods encode a value into a string of the solution's own format and decode it back (LeetCode 297, 449, 535) is judged on `decode(encode(x))`, never on the string. `judge.round_trip` declares it:

| Field | Type | Required | Default | Constraints | Since |
|---|---|---|---|---|---|
| `judge.round_trip.encode` | string | yes | | A method with exactly one parameter, returning `string` | 1 |
| `judge.round_trip.decode` | string | yes | | Another method, with exactly one parameter of type `string`, returning the type of `encode`'s parameter | 1 |
| `judge.round_trip.instances` | string | yes | | `separate` or `same` | 1 |

- `round_trip` is for class problems only, and the constructor MUST take no parameters.
- A case gives the value to encode as an object keyed by `encode`'s parameter name, instead of `ops` and `args`; `expected` is `decode`'s result (see [Cases](#cases)).
- For each case the instance or instances are constructed, `encode` is called with the value, `decode` with the string `encode` returned, and `decode`'s result is judged with `judge.compare`. `encode` returning something other than a string is a Runtime Error.
- `separate`: `decode` runs on a second instance, as in LeetCode's `ser = Codec(); deser = Codec(); deser.deserialize(ser.serialize(root))`, so the solution cannot pass the value through instance state. `same`: both calls run on one instance (535: `codec.decode(codec.encode(url))`).
- Both instances live in the same process. Other methods are never called, and requirements on the encoding itself (449: "as compact as possible") are not judged.

Problem 0297 (`separate`; 0535 is `same`):

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
    instances: separate
```

```json
{"input": {"root": [1,2,3,null,null,4,5]}, "expected": [1,2,3,null,null,4,5]}
```

## Cases

`cases.jsonl` holds one case per line, a JSON object with these fields and no others:

| Field | Type | Required | Meaning | Since |
|---|---|---|---|---|
| `input` | object | yes | The case's input, by kind (below) | 1 |
| `expected` | JSON value | no | The expected output, by kind (below). `null` is a value: `"expected": null` is an expected output, a missing field is none | 1 |
| `example` | boolean | no | The case is an example from the statement | 1 |
| `note` | string | no | A description of the case | 1 |

- A line holds exactly one JSON value. Lines that are blank, or start with `//` after leading whitespace, are ignored.
- Cases are numbered from 1, counting only the lines that hold a case.
- A case without `expected` is valid but cannot be judged ([Verdicts](#verdicts)).

`input` and `expected` by kind:

| Kind | `input` | `expected` |
|---|---|---|
| Function | One value per parameter and hidden input, keyed by name, and no other keys. A `ref` parameter takes the referenced node's value, and a `copy` parameter is absent ([Structure inputs](#structure-inputs)) | The judged value: the return value, the value of `judge.output` ([In-place output](#in-place-output)), or a node reference (`returns_ref`) |
| Class | `{"ops": [...], "args": [...]}` and no other keys. `ops` is a non-empty array of names: `ops[0]` is the class name (the constructor), the others are method names. `args` has one array per op, holding exactly that op's arguments | An array with one entry per op: `null` for the constructor and for `void` methods, the return value otherwise |
| Round trip | One key, `encode`'s parameter name, with the value to encode | `decode`'s result |

With `judge.compare: any_of`, `expected` is instead an array of all acceptable values of the form above. With `checker`, it is one valid answer. Values MUST have the encoding of their declared type ([Types](#types)); a `NextTreeNode` output has its level form.

Problem 0146:

```json
{"input": {"ops": ["LRUCache","put","put","get","put","get","put","get","get","get"], "args": [[2],[1,1],[2,2],[1],[3,3],[2],[4,4],[1],[3],[4]]}, "expected": [null,null,null,1,null,-1,null,-1,3,4], "example": true}
```

## Judging

A case is judged by comparing the solution's output with `expected` under `judge.compare`, or by the problem's checker.

### Compare modes

| `judge.compare` | The output is accepted when | Since |
|---|---|---|
| `exact` | It equals `expected` | 1 |
| `unordered` | `expected` and the output are arrays that are equal as multisets: the order of the outermost array is ignored | 1 |
| `unordered_deep` | As `unordered`, with the order also ignored in every nested array, at every level | 1 |
| `any_of` | It equals one element of `expected`, an array of the acceptable answers | 1 |
| `checker` | `checker.py` accepts it; `expected` is one valid answer passed to it ([Checkers](#checkers)) | 1 |

- Equality is equality of JSON values: objects by their keys and values, arrays element by element, strings, booleans and `null` exactly. Two numbers that are both integers (written without fraction or exponent, in the signed 64-bit range) are equal when their values are equal. Other pairs of numbers are equal when, with tolerance t (`judge.tolerance`), \|a − b\| ≤ t or \|a − b\| ≤ t · max(\|a\|, \|b\|); with t = 0, when their values are equal (`2` equals `2.0`).
- The tolerance applies in every mode.
- `unordered` and `unordered_deep` require the judged value of a function or round-trip problem to be an array type. In a class problem the judged value is the array of results, and its order is what they ignore.
- A class problem that is not a round trip MUST NOT use `any_of`: its `expected` holds one entry per op.

### In-place output

For function problems that modify an input:

- `judge.output: <input>` names a parameter or hidden input. The judged value is its value after the call, encoded with its type, instead of the return value. Problem 0189 (`rotate`, returning `void`) judges `nums`.
- `judge.output_length: return` judges only the first k elements of `judge.output`, k being the return value (LeetCode's "the elements beyond the first k do not matter"). It requires `judge.output`, an array type for it, and a return type of `integer` or `long`. A returned k outside 0 to the array's length is a Runtime Error.

Problem 0027:

```yaml
function:
  name: removeElement
  params:
    - {name: nums, type: "integer[]"}
    - {name: val, type: integer}
  returns: integer
judge:
  compare: unordered
  output: nums
  output_length: return
```

```json
{"input": {"nums": [3,2,2,3], "val": 3}, "expected": [2,2]}
```

### Timeout

`judge.timeout` is a wall-clock limit for one whole run of a solution over the cases selected for it, not a per-case limit. It guards against hangs and infinite loops; it does not judge complexity. When it expires the run is stopped, and the cases without a result are Time Limit Exceeded; cases that finished keep their verdicts. A checker run gets its own limit of the same duration. The default is the workspace's `timeout` (10s unless configured).

### Readiness

A problem is **ready** when it has `judge.compare`. `judge.compare` has no default, and MUST be written out even for `exact`. A problem that is not ready is valid, but its judge rule is undecided: its cases are not judged (Not Judged), and it cannot be submitted.

### Verdicts

A run of a solution has one of these outcomes for each case:

| Verdict | When |
|---|---|
| Compile Error | The solution does not build: a Go compile error, a Python syntax error. This is the outcome of the whole run, and no case runs |
| Runtime Error | The case raised an error (an exception, a panic, an input the driver cannot decode, or a result the encoding cannot express), or the program exited before producing the case's result |
| Time Limit Exceeded | The [timeout](#timeout) expired before the case produced its result |
| No Expected Output | The case ran, but has no `expected` |
| Not Judged | The case ran, but the problem is not [ready](#readiness) |
| Wrong Answer | The output does not match `expected` under `judge.compare`, or the checker rejected it |
| Accepted | The output matches, or the checker accepted it |

The first three are decided before judging; No Expected Output is decided before Not Judged. The verdict of the whole run is the first case, in case order, whose verdict is Runtime Error, Time Limit Exceeded or Wrong Answer. Without one, it is Accepted if any case was accepted (or no case ran), else Not Judged if any case was not judged, else No Expected Output.

A broken checker is not a verdict: see [Checkers](#checkers).

## Checkers

A problem with `judge.compare: checker` MUST have `checker.py` in its directory, and a problem with any other mode (or none) MUST NOT. Checkers are written in Python whatever the solution's language, and MUST work on Python 3.10.

```python
def check(input, output, expected):
    """Return None or True to accept; False or a message string to reject."""
```

`checker.py` defines `check` at module level. Its arguments are decoded JSON (dicts, lists, strings, numbers, booleans, `None`):

| Argument | Function problem | Class problem | Round trip |
|---|---|---|---|
| `input` | The case's `input` object: parameters and hidden inputs by name (`ref` values as given, no `copy` parameters) | `{"ops": [...], "args": [...]}` | `{<encode's parameter>: value}` |
| `output` | The judged value, as in `expected` | The array of results, one per op | `decode`'s result |
| `expected` | The case's `expected`, one valid answer | | |

- Return values: `None` or `True` accepts; `False` rejects; a string rejects with that string as the message. A rejected case is a Wrong Answer, and its message is shown with it. Messages SHOULD say what is wrong: `"'bb' is not a substring of s"`.
- `check` is called once per case that produced an output and has `expected`, in the order the cases ran, in one process that runs after the solution, with the [timeout](#timeout)'s duration as its own limit. Cases that already failed (Runtime Error, Time Limit Exceeded) or have no `expected` are not passed.
- A checker that cannot be loaded, has no callable `check`, raises, returns anything else, or runs out of time is a broken problem, not a verdict on the solution: the run fails with an error naming `checker.py` and the case.
- A checker MAY import the public names of `hc_runtime` ([Python](#python)) to rebuild structures, `decode("TreeNode", output)`, and otherwise only the standard library.
- A checker MUST accept every correct output. For randomized problems (LeetCode 384, 528), whose outputs are checked for a plausible distribution, this means a correct solution MUST be rejected with negligible probability.

Problem 0005 (Longest Palindromic Substring, `"bab"` and `"aba"` both right):

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

## Solution ABI

What a solution must look like for the drivers to call it. User solutions, reference solutions (`reference/solution.*`) and templates (`templates/solution.*`) are all solutions. References and templates MUST follow this section and use only the standard library; templates MUST keep the signature, with the function, class and method names, parameter order and types given here. A solution receives the parameters in the order of `function.params` (or of the constructor and method parameters), without hidden inputs, bound as in [Types](#types).

### Python

The solution is `solution.py`.

| | Function problem | Class problem |
|---|---|---|
| Entry point | A class `Solution` with a method named `function.name`, instantiated without arguments for each case; or, when the module defines no `Solution`, a module-level function named `function.name` | A class named `class.name`, whose `__init__` takes the constructor parameters and whose methods have the names in `class.methods` |

- An uncaught exception is a Runtime Error; a syntax error is a Compile Error.
- The protocol's types come from the module `hc_runtime`. Its public API is exactly the names in its `__all__`: `ListNode`, `TreeNode`, `GraphNode`, `NaryNode`, `RandomListNode`, `NextTreeNode`, `QuadNode`, `MultilevelNode`, `NestedInteger`, `Employee`, `decode` and `encode`. Solutions and checkers MAY import these and MUST NOT import anything else from it.
- The problem's `Node` type is imported under that name: `from hc_runtime import GraphNode as Node`. The others keep their names: `from hc_runtime import ListNode, TreeNode`, `from hc_runtime import NestedInteger`.
- `decode(type, value)` turns a JSON value into the Python value for the protocol type `type` (a canonical name such as `"TreeNode"` or `"integer[]"`), and `encode(type, value)` does the reverse.
- A reference MUST import the names it uses. A template MAY omit the imports: a template that does not mention `hc_runtime` gets the standard import block (`typing` names, `ListNode`, `TreeNode`, and the problem's `Node`, `NestedInteger` and `Employee`) put in front of it.

### Go

The solution is `solution.go`, in `package main`.

| | Function problem | Class problem |
|---|---|---|
| Entry point | `func <function.name>(<params>) <result>`; no result for `void` | `type <class.name> struct`, `func Constructor(<constructor params>) <class.name>` (or `*<class.name>`), and a method for each entry of `class.methods`, with pointer receivers as in the starters |

- Method names start with an upper-case letter (`getMin` becomes `GetMin`), as in LeetCode's Go templates, except a round trip's `encode` and `decode` methods, which keep their names (`serialize`).
- The driver predefines `ListNode` and `TreeNode` in every problem, and the problem's `Node` type, `NestedInteger` and `Employee` in the problems that use them. Solutions MUST NOT declare them.
- `main` and package-level identifiers starting with `hc` are reserved.
- The solution is a module of its own without dependencies, so it can import only the standard library.
- A panic is a Runtime Error; a build failure is a Compile Error.
- A template without a `package` clause gets `package main` put in front of it.

### Language versions

| Language | Oldest supported version |
|---|---|
| Python | 3.10 |
| Go | 1.22 |

Reference solutions, templates and checkers MUST work on these versions, language and standard library: a reference's expected outputs are only computed, and it is only checked, on a supported toolchain. User solutions MAY use anything their toolchain supports. To check Go code against Go 1.22, give its module `go 1.22` and run `go vet`, whose `stdversion` check reports newer standard library symbols.

## Validity rules

A valid pack satisfies every rule below, and so does each of its problems. `hc validate` checks the rules on `problem.yaml` and `cases.jsonl`, and the pack's rules on problem directories and identity; `hc validate --run` also checks the rules on reference solutions.

**problem.yaml**

1. It MUST be valid YAML, and MUST NOT contain unknown fields.
2. `slug` MUST match `^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`; `title` MUST NOT be empty; `difficulty`, when present, MUST be `easy`, `medium` or `hard` (in any case).
3. `kind` MUST be `function` or `class`, the matching section MUST be present, with a `name`, and the other section MUST NOT be present. A class MUST have at least one method, and method names MUST be non-empty and unique, also when their first letters are capitalized.
4. Parameter names MUST be non-empty and unique: across `function.params` and `function.hidden` together, within the constructor, and within each method.
5. Every type MUST be a [supported type](#types) within the array limits; `void` MUST only be a return type. A problem MUST NOT use more than one `Node` type.
6. `cycle`, `join`, `ref` and `copy` MUST only appear on function parameters, at most one per parameter, and MUST follow the [structure input rules](#structure-inputs), as MUST `hidden` and `returns_ref`.
7. `returns_copy` MUST follow the [returns_copy rules](#returns_copy).
8. `judge.output` and `judge.output_length` MUST only appear in function problems. `judge.output`, when present, MUST name a parameter or hidden input without a `cycle`; a `void` function MUST have `judge.output`.
9. `judge.output_length`, when present, MUST be `return`, with `judge.output` naming an array and an `integer` or `long` return type.
10. `judge.round_trip` MUST only appear in class problems, and MUST follow the [round-trip rules](#round-trip-problems).
11. `judge.compare`, when present, MUST be one of the [compare modes](#compare-modes). With `unordered` or `unordered_deep`, the judged value of a function or round-trip problem MUST be an array. A class problem that is not a round trip MUST NOT use `any_of`.
12. `judge.tolerance`, when present, MUST be a finite number, 0 or more.
13. `checker.py` MUST be present with `judge.compare: checker`, and absent otherwise.
14. `judge.timeout`, when present, MUST be a positive duration.

**cases.jsonl**

1. Every line that is not blank or a comment MUST be one JSON object with `input` (an object) and, optionally, `expected`, `example` (a boolean) and `note` (a string), and no other fields.
2. A function case's `input` MUST have exactly the keys of the parameters (except `copy` parameters) and hidden inputs; a class case's exactly `ops` and `args`; a round-trip case's exactly `encode`'s parameter.
3. Every value MUST have the encoding of its type ([Types](#types)), including the constraints of the tree and judge-defined types: a `TreeNode` level order with children only for its nodes, neighbors in the graph, random indices in the list, well-formed `NaryNode`, `QuadNode` and `MultilevelNode` forms, distinct employee ids and known subordinates. A `ref` value MUST be an integer.
4. Structure inputs MUST fit: a `cycle` position in the list, joined lists that end with the same values from `skip` and `at`, a `ref` value that occurs exactly once.
5. In a class case, `ops` and `args` MUST have the same, non-zero length, `ops[0]` MUST be the class name, every other op MUST be a method, and every `args` entry MUST hold exactly the op's arguments, of their types.
6. `expected`, when present, MUST have the form of the judged value ([Cases](#cases)): for `any_of`, an array of such values; for a class case, one entry per op, `null` for the constructor and `void` methods. Its values MUST be encoded as outputs are, which rule 3 does not require of inputs: no `null` for an empty `ListNode`, `TreeNode` or `Node`, no trailing `null`s, and `[0,1]` for a `QuadNode` that is not a leaf.

**pack.yaml**

1. It MUST be valid YAML, and MUST NOT contain unknown fields.
2. `protocol` MUST be a positive integer, and the pack MUST NOT use a feature introduced after it ([Protocol versions](#protocol-versions)).
3. `name` MUST match `^[a-z0-9]+(-[a-z0-9]+)*$`, and MUST NOT be `local`.
4. `version`, when present, MUST be a SemVer 2.0.0 version without a `v` prefix.
5. `languages`, when present, MUST NOT be empty, and MUST hold language ids of the declared protocol, each once.
6. Every group MUST have a `name` matching `^[a-z0-9]+(-[a-z0-9]+)*$`, unique among the groups, and MUST NOT list a slug twice.

**The pack**

1. `pack.yaml` MUST be at the root of the pack, and the problems in `problems/` ([Packs](#packs)).
2. Every problem directory MUST have the canonical name ([Identity](#identity)).
3. `slug` MUST be unique in the pack, and so MUST `id` when present.
4. Every entry of a group MUST be the slug of a problem of the pack.

**References, templates and checkers**

1. Every reference solution MUST build, and MUST pass every case that has `expected` under the problem's judge rule (Accepted, or Not Judged for a problem that is not ready), on a supported toolchain; cases without `expected` MUST run without error. With a checker, the checker MUST NOT break on any reference output.
2. References, templates and checkers MUST follow the [Solution ABI](#solution-abi) and [Checkers](#checkers), and work on the [oldest supported versions](#language-versions). `hc validate` does not check this rule.
