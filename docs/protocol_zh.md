# handcoding 题目协议

[English](protocol.md) · 简体中文

> 本文是 [docs/protocol.md](protocol.md) 的译文。英文原文是规范文本，两者不一致时以英文为准。`hc docs protocol <id>` 打印英文原文的一节，节的 id 来自英文标题。

## 范围与版本

本文规定 handcoding 的题目协议：题目在磁盘上如何组织，各个文件的含义，用例如何评判，以及题目附带的代码（参考解、初始代码、判题程序）可以依赖什么。本文是规范性的。

MUST、MUST NOT、SHOULD、MAY 按 RFC 2119 的含义使用，译文中分别写作"必须"、"不得"、"应当"、"可以"。违反"必须"的题目是不合法的；[合法性规则](#合法性规则)汇总了所有这类规则。

这是**协议 1**。下文的每个字段、类型和模式，除非另有说明，都属于协议 1。题目包在清单中用 `protocol: N` 声明它所依据的协议（见[题目包](#题目包)）；更新的协议可能增加特性，旧的题目包不会用到它们。

hc 自身的行为（`hc test` 运行哪些用例、`hc fetch` 如何导入题目）记录在 `docs/how-to-use.md` 中，不在本文。hc 与各语言驱动程序之间如何通信是 hc 的内部细节，记录在 `CONTRIBUTING.md` 中。

### 协议版本

| 版本 | 变更 |
|---|---|
| 1 | 最初冻结的协议。 |

### 不在范围内

协议 1 不定义以下内容，它们都将以后续的增量变更加入：

- 交互题，即解答要与评测程序对话的题目（issue #7）；
- `tests` 题，用测试代码而不是用例评判（issue #31）；
- 用例生成器，以及性能评判（issue #33）；
- Go 和 Python 以外的解答语言。

## 题目包

本节是占位内容。题目包及其清单 `pack.yaml` 将随 issue #27 在协议 1 中规定。在此之前，一个工作区的 `problems/` 目录就是一个题库，下文所说的"题库"都指它。

## 题目目录

一道题目是一个目录，包含以下条目：

| 条目 | 必需 | 内容 |
|---|---|---|
| `problem.yaml` | 是 | 元数据、签名和判题选项；见 [problem.yaml](#problemyaml) |
| `description.md` | 否 | 题面，Markdown 格式 |
| `assets/` | 否 | 题面引用的文件，如图片 |
| `cases.jsonl` | 否 | 测试用例；见[用例](#用例) |
| `reference/solution.py`、`reference/solution.go` | 否 | 参考解，可以只有其中一部分语言 |
| `templates/solution.py`、`templates/solution.go` | 否 | 初始代码，取代根据签名生成的初始代码 |
| `checker.py` | 仅限 `judge.compare: checker` | 判题程序；见[判题程序](#判题程序) |

- 没有 `description.md` 的题目题面为空，没有 `cases.jsonl` 的题目没有用例。两者都合法，但这样的题目没法真正拿来练习。
- 题面用相对于题目目录的路径引用本地文件，这些文件必须位于 `assets/` 下：`![](assets/rev1ex1.jpg)`。
- 其他条目不属于题目。工具可以忽略或丢弃它们。
- 参考解、初始代码和判题程序可以包含什么，由[解答 ABI](#解答-abi) 和[判题程序](#判题程序)规定。

### 标识

- `slug` 是题目的标识，在所属题库中必须唯一。题解和提交历史以它为键（通过目录名），所以修改 slug 是破坏性变更。
- `id` 可选。有 `id` 时，它在所属题库中必须唯一。它用于排序和查找。
- 目录必须使用规范名称：`id` 是十进制整数 n 时为 `<n 补足 4 位>-<slug>`（`id: "1"`、slug 为 `two-sum` 时是 `0001-two-sum`；`id: "12345"` 时是 `12345-<slug>`）；否则为 `<slug>`。

## problem.yaml

`problem.yaml` 是一个 YAML 映射。不认识的字段在任何层级都是错误。

| 字段 | 类型 | 必需 | 默认值 | 约束 | 起始版本 |
|---|---|---|---|---|---|
| `id` | 字符串 | 否 | 无 | 在题库中唯一；见[标识](#标识)。YAML 整数按其十进制文本读取 | 1 |
| `slug` | 字符串 | 是 | | 字母、数字和单个短横线：`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`。应当全部小写。在题库中唯一 | 1 |
| `title` | 字符串 | 是 | | 非空 | 1 |
| `difficulty` | 字符串 | 否 | 无 | `easy`、`medium` 或 `hard`，大小写不限 | 1 |
| `tags` | 字符串列表 | 否 | `[]` | | 1 |
| `kind` | 字符串 | 是 | | `function` 或 `class`。不会根据出现了哪些段来推断 | 1 |
| `function` | 映射 | `kind: function` 时 | | 见[函数题](#函数题) | 1 |
| `class` | 映射 | `kind: class` 时 | | 见[设计题](#设计题) | 1 |
| `judge.compare` | 字符串 | 否 | 无 | `exact`、`unordered`、`unordered_deep`、`any_of` 或 `checker`；见[比较方式](#比较方式)。没有它，题目就不是[就绪](#就绪)的 | 1 |
| `judge.tolerance` | 数字 | 否 | 被评判的值（不是编解码往返题的设计题中为方法的返回值）的基础类型为 `double` 时为 `1e-5`，否则为 `0` | 有限的数，不小于 0。`0` 表示精确比较数字。见[比较方式](#比较方式) | 1 |
| `judge.output` | 字符串 | `void` 函数必需 | 无 | 仅限函数题：一个参数或隐藏输入；见[原地修改的输出](#原地修改的输出) | 1 |
| `judge.output_length` | 字符串 | 否 | 无 | 仅限函数题：`return`；见[原地修改的输出](#原地修改的输出) | 1 |
| `judge.round_trip` | 映射 | 否 | 无 | 仅限设计题：`encode`、`decode`、`instances`；见[编解码往返题](#编解码往返题) | 1 |
| `judge.timeout` | 字符串 | 否 | 工作区的 `timeout`（未配置时为 10s） | 正的时长，如 `5s` 或 `500ms`；见[时限](#时限) | 1 |
| `source.site` | 字符串 | 否 | 无 | 任意文本，如 `leetcode.com` | 1 |
| `source.url` | 字符串 | 否 | 无 | | 1 |

与 `kind` 不对应的段（函数题中的 `class`、设计题中的 `function`）不得出现。

> **YAML 的坑。** 在 YAML 的流式写法（`{...}`、`[...]`）中，`[]` 本身是 YAML 语法，所以其中的数组类型必须加引号：`{name: nums, type: "integer[]"}`。用块状写法（`type: integer[]` 单独一行）时，引号可加可不加。

内置示例题 0001：

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

## 类型

类型名沿用 LeetCode `metaData` 中的名字。一个类型是一个基础类型，后面跟零个或多个 `[]`。以下别名会被接受并规范化：`list<T>` 规范化为 `T[]`（可嵌套：`list<list<integer>>` 即 `integer[][]`），`int` 为 `integer`，`bool` 为 `boolean`，`char` 为 `character`，`float` 为 `double`。

用例、期望输出和解答输出中的每个值都是 JSON。本节为每个类型规定它的 JSON 编码，以及它在各解答语言中的绑定。除非某个类型另有说明，输入（用例中的 `input`）和输出（`expected`，以及解答结果编码成的值）的编码相同。

| 类型 | JSON | Go | Python | 起始版本 |
|---|---|---|---|---|
| `integer` | 整数 | `int` | `int` | 1 |
| `long` | 整数 | `int64` | `int` | 1 |
| `double` | 数字 | `float64` | `float` | 1 |
| `boolean` | `true` 或 `false` | `bool` | `bool` | 1 |
| `string` | 字符串 | `string` | `str` | 1 |
| `character` | 一个字节的字符串 | `byte` | `str` | 1 |
| `ListNode` | 见 [ListNode 和 TreeNode](#listnode-和-treenode) | `*ListNode` | `ListNode \| None` | 1 |
| `TreeNode` | 见 [ListNode 和 TreeNode](#listnode-和-treenode) | `*TreeNode` | `TreeNode \| None` | 1 |
| `GraphNode`、`NaryNode`、`RandomListNode`、`NextTreeNode`、`QuadNode`、`MultilevelNode` | 见 [LeetCode 的 Node 类型](#leetcode-的-node-类型) | `*Node` | `Node \| None` | 1 |
| `NestedInteger` | 见 [NestedInteger 和 Employee](#nestedinteger-和-employee) | `*NestedInteger` | `NestedInteger` | 1 |
| `Employee` | 见 [NestedInteger 和 Employee](#nestedinteger-和-employee) | `*Employee` | `Employee` | 1 |
| `T[]` | 见[数组](#数组) | `[]T` | `list[T]` | 1 |
| `void` | 无；只能作返回类型 | 没有返回值 | `None` | 1 |

### 标量

- `integer` 和 `long`：不带小数部分或指数的 JSON 数字（`3`，而不是 `3.0` 或 `3e0`），在有符号 64 位范围内。协议不把 `integer` 限制为 32 位。
- `double`：任意 JSON 数字。整数也是合法的 double（`2`、`2.0`）。
- `boolean`：`true` 或 `false`。
- `string`：JSON 字符串，可以包含任意 Unicode 文本。
- `character`：恰好一个字节的 JSON 字符串，即一个 ASCII 字符：`"a"`。

### 数组

- `T[]` 是元素为 `T` 类型值的 JSON 数组。
- 数组最多嵌套 3 层（`integer[][][]`）。`ListNode`、`TreeNode`、`NestedInteger` 和 `Employee` 的数组最多 1 层。`Node` 类型不能出现在数组中。
- 解答返回的数组结果是 Go 的 `nil` 或 Python 的 `None` 时，编码为 `[]`。

### ListNode 和 TreeNode

Go：`type ListNode struct { Val int; Next *ListNode }` 和 `type TreeNode struct { Val int; Left, Right *TreeNode }`。Python：`ListNode(val=0, next=None)` 和 `TreeNode(val=0, left=None, right=None)`，属性为 `val`、`next`、`left`、`right`。节点的值是整数。

- `ListNode`：从头节点开始的值组成的数组，`[1,2,3]`。`[]` 是空链表。在输入中，`null` 也表示空链表。
- `TreeNode`：层序数组，缺失的子节点为 `null`：`[3,9,20,null,null,15,7]`。第一个元素是根的值，之后每两个元素依次给出层序中下一个（非 `null`）节点的左右子节点。输出会去掉末尾的 `null`。`[]` 是空树；在输入中，`null` 和 `[null]` 也表示空树。为多于实际节点数的节点列出子节点的数组是不合法的，如 `[1,null,null,2]` 或 `[null,1]`。

### LeetCode 的 Node 类型

LeetCode 把六种不同的结构都叫作 `Node`。协议给每种结构各起一个类型名。在解答代码中，每种都是名为 `Node`、字段名与 LeetCode 一致的类型（Go）或类（Python），所以一道题不得使用其中一种以上。

| 类型 | 字段（Go；Python 用同名但首字母小写的字段） | JSON | LeetCode 题目 |
|---|---|---|---|
| `GraphNode` | `Val int`、`Neighbors []*Node` | 节点值为 1 到 n 的图的邻接表：第 i 项（从 0 开始）按值列出值为 i+1 的节点的邻居，该类型的值是值为 1 的节点。`[[2,4],[1,3],[2,4],[1,3]]`；`[[]]` 是单个节点，`[]` 表示没有节点。每个邻居都必须在 1 到 n 之间 | 133 |
| `NaryNode` | `Val int`、`Children []*Node` | 层序：根的值、`null`，然后按层序依次写出每个节点的子节点，每组后面跟一个 `null`，去掉末尾的 `null`：`[1,null,3,2,4,null,5,6]`。`[]` 表示没有树。组数不得多于节点数 | 429、559、589、590 |
| `RandomListNode` | `Val int`、`Next *Node`、`Random *Node` | 按链表顺序写出的 `[[val, randomIndex], ...]`，`randomIndex` 是 `Random` 所指节点的下标（从 0 开始），或 `null`：`[[7,null],[13,0],[11,4],[10,2],[1,0]]`。每个下标都必须在链表内 | 138 |
| `NextTreeNode` | `Val int`、`Left *Node`、`Right *Node`、`Next *Node` | 输入：与 `TreeNode` 相同的层序，所有 `Next` 指针都为空。输出（以及 `expected`）：一个 `integer[][]`，即从每层最左边的节点出发、沿 `Next` 读出的各层的值：`[[1],[2,3],[4,5,7]]` | 116、117 |
| `QuadNode` | `Val bool`、`IsLeaf bool`、`TopLeft`、`TopRight`、`BottomLeft`、`BottomRight *Node` | 层序，每个节点都有四个子节点位置，空位为 `null`，每个节点写成 `[isLeaf, val]`，取 `0` 或 `1`，去掉末尾的 `null`：`[[0,1],[1,0],[1,1],[1,1],[1,0]]`。非叶子节点不论取什么值，输出时都写成 `[0,1]` | 427、558 |
| `MultilevelNode` | `Val int`、`Prev *Node`、`Next *Node`、`Child *Node` | 各层依次写出，每层以 `null` 结束；子层前面再加若干个 `null`，个数等于其父节点在上一层中的下标（从 0 开始）：`[1,2,3,4,5,6,null,null,null,7,8,9,10,null,null,11,12]` | 430 |

- 在输入中，`null` 对每种 `Node` 类型都表示没有节点。
- Python 构造函数的参数与 LeetCode 一致，且都可省略：`Node(val, neighbors)`、`Node(val, children)`、`Node(x, next, random)`（存为 `val`）、`Node(val, left, right, next)`、`Node(val, isLeaf, topLeft, topRight, bottomLeft, bottomRight)`、`Node(val, prev, next, child)`。
- 解答可以返回自己创建的节点。返回的结构无法用这种编码表达时，算运行错误：`GraphNode` 不是一个值为 1 到 n（各出现一次）的图中的节点 1；随机指针指向返回链表之外的节点；`MultilevelNode` 的 `Prev` 没有指向前一个节点，或者某层第一个节点的 `Prev` 不为空；同一层有两个节点带子层。

### NestedInteger 和 Employee

- `NestedInteger` 存放一个整数，或一个由 `NestedInteger` 组成的列表。JSON：一个整数，或由 `NestedInteger` 值组成的数组：`[1,[4,[6]]]`。`NestedInteger[]` 是它们组成的数组（内置示例题 0341：`[[1,1],2,[1,1]]`）。接口与 LeetCode 一致，所以解答也可以自己构造一个（LeetCode 385）：
  - Go：`IsInteger() bool`、`GetInteger() int`、`SetInteger(int)`、`Add(NestedInteger)`、`GetList() []*NestedInteger`。零值是空列表。
  - Python：`NestedInteger(value=None)`（不给值就是空列表）、`isInteger()`、`getInteger()`、`setInteger(value)`、`add(elem)`、`getList()`。
- `Employee` 的字段是 `Id int`、`Importance int`、`Subordinates []int`（Python：`Employee(id, importance, subordinates)`）。JSON：`[id, importance, [下属 id]]`。`Employee[]` 中的 id 必须互不相同，每个下属都必须是其中之一：`[[1,5,[2,3]],[2,3,[]],[3,3,[]]]`（LeetCode 690）。

## 签名

`kind: function` 的题目要求实现一个函数；`kind: class` 的题目要求实现一个由一串操作驱动的类（LeetCode 的设计题）。参数是一个映射 `{name, type}`；参数名必须非空，并且在所属列表中唯一（函数题中 `params` 和 `hidden` 合在一起算），参数类型不得是 `void`。

### 函数题

| 字段 | 类型 | 必需 | 默认值 | 约束 | 起始版本 |
|---|---|---|---|---|---|
| `function.name` | 字符串 | 是 | | 解答定义的函数名 | 1 |
| `function.params` | 参数列表 | 否 | `[]` | 按此顺序传给解答。每个参数可以带 `cycle`、`join`、`ref`、`copy` 之一；见[结构输入](#结构输入) | 1 |
| `function.hidden` | 参数列表 | 否 | `[]` | 不传给解答的用例输入；见[结构输入](#结构输入)。名字必须与参数不同 | 1 |
| `function.returns` | 类型 | 否 | `void` | | 1 |
| `function.returns_ref` | 字符串 | 否 | 无 | 见[结构输入](#结构输入) | 1 |
| `function.returns_copy` | 字符串 | 否 | 无 | 见 [returns_copy](#returns_copy) | 1 |

被评判的值是返回值，或者调用结束后 `judge.output` 的值（[原地修改的输出](#原地修改的输出)）。`void` 函数必须有 `judge.output`。

### 设计题

| 字段 | 类型 | 必需 | 默认值 | 约束 | 起始版本 |
|---|---|---|---|---|---|
| `class.name` | 字符串 | 是 | | 解答定义的类名 | 1 |
| `class.constructor` | 参数列表 | 否 | `[]` | 构造函数的参数 | 1 |
| `class.methods` | 方法列表 | 是 | | 至少一个 | 1 |
| `class.methods[].name` | 字符串 | 是 | | 非空，且在各方法中唯一，首字母大写后也唯一（`get` 和 `Get` 在 Go 中是同一个方法） | 1 |
| `class.methods[].params` | 参数列表 | 否 | `[]` | | 1 |
| `class.methods[].returns` | 类型 | 否 | `void` | | 1 |

一个用例给出一串操作：先是构造函数，然后是对它所创建实例的方法调用。被评判的值是这些操作的结果组成的数组；见[用例](#用例)。设计题没有[结构输入](#结构输入)，也不得有 `judge.output` 或 `judge.output_length`。

内置示例题 0146：

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

### 结构输入

有些函数题传给解答的链式结构无法用一个 JSON 值描述：尾部连回自身的链表、共享尾部的两个链表、树中的某个节点。以下属性用来描述它们：

| 属性 | 位置 | 含义 | 起始版本 |
|---|---|---|---|
| `hidden` | `function` | 用来构造参数或用于评判、但不传给解答的用例输入（`{name, type}`） | 1 |
| `cycle: <输入>` | `ListNode` 参数 | 链表的最后一个节点连到下标为 `<输入>` 的节点，`<输入>` 是一个 `integer` 类型的隐藏输入（从 0 开始）；`-1` 表示没有环 | 1 |
| `join: {list, skip, at}` | `ListNode` 参数 | 链表保留自己的前 `skip` 个节点，然后接上参数 `list` 从下标 `at` 开始的节点：是同一批节点，不是拷贝。`skip` 和 `at` 是 `integer` 类型的隐藏输入 | 1 |
| `ref: <输入>` | `ListNode` 或 `TreeNode` 参数 | 用例给出一个节点的值；解答收到的是结构 `<输入>` 中存放该值的节点 | 1 |
| `copy: <参数>` | `ListNode` 或 `TreeNode` 参数 | 用例中不给出；解答收到参数 `<参数>` 的深拷贝 | 1 |
| `returns_ref: <输入>` | `function` | 函数返回结构 `<输入>` 中的一个节点（或不返回节点），按节点身份评判 | 1 |

规则：

- 一个参数最多带 `cycle`、`join`、`ref`、`copy` 之一。隐藏输入不带这些属性。
- `cycle`、`join.skip` 和 `join.at` 指向 `integer` 类型的隐藏输入。
- `join.list` 指向另一个类型为 `ListNode`、且不带这四个属性的参数。
- `ref` 指向另一个类型相同、本身不是 `ref` 的参数或隐藏输入。
- `copy` 指向另一个类型相同、不带这四个属性的参数（不能是隐藏输入）。
- `returns_ref` 要求返回类型为 `ListNode` 或 `TreeNode`，并指向该类型的、不是 `ref` 的参数或隐藏输入。它与 `judge.output` 互斥。
- 每个隐藏输入都被某个 `cycle`、`join`、`ref`、`returns_ref` 或 `judge.output` 用到。
- `judge.output` 不得指向带 `cycle` 的参数。

在用例中：

- `cycle` 输入在 `-1` 到链表长度减 1 之间。
- 对于 `join`，`skip` 在 0 到被接上的参数的长度之间，`at` 在 0 到 `list` 的长度之间，并且该参数从下标 `skip` 开始的值与 `list` 从下标 `at` 开始的值相同。`at` 等于 `list` 的长度（从而 `skip` 等于该参数的长度）表示两个链表不相交。
- `ref` 参数的值是一个整数，在它所指结构的值中恰好出现一次（所指结构是 `copy` 时，看它的源结构的值）。
- 没有 `copy` 参数。

`returns_ref` 按节点身份编码返回的节点，输出和 `expected` 都这样写：在 `ListNode` 结构中是它按节点顺序的下标（从 0 开始），在 `TreeNode` 结构中是它的值，没有节点时为 `null`。返回不属于该结构的节点算运行错误。

内置示例题 0142（环形链表 II）给出 `head` 和 `pos`，解答只收到 `head`：

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

内置示例题 0160（相交链表）：`headA` 和 `headB` 是包括共享尾部在内的完整链表：

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

内置示例题 0236（二叉树的最近公共祖先）：`p` 和 `q` 在用例中是值，在解答中是 `root` 的节点：

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

内置示例题 0237（删除链表中的节点）引用一个隐藏输入，并评判它：

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

内置示例题 1379 的用例中没有 `cloned`：

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

`function.returns_copy: <参数>` 表示函数返回参数 `<参数>` 的深拷贝（LeetCode 的克隆图、随机链表的复制）。如果返回的结构与该参数的结构共享任何节点，该用例算运行错误。

- `<参数>` 指向一个类型与返回类型相同的参数（不能是隐藏输入）。
- 返回类型是 `ListNode`、`TreeNode` 或某种 `Node` 类型。
- `returns_copy` 与 `returns_ref` 和 `judge.output` 互斥。

内置示例题 0133：

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

### 编解码往返题

如果设计题的方法把一个值编码成解答自定格式的字符串，再解码回来（LeetCode 297、449、535），评判的是 `decode(encode(x))`，而不是那个字符串。`judge.round_trip` 声明这类题目：

| 字段 | 类型 | 必需 | 默认值 | 约束 | 起始版本 |
|---|---|---|---|---|---|
| `judge.round_trip.encode` | 字符串 | 是 | | 恰好有一个参数、返回 `string` 的方法 | 1 |
| `judge.round_trip.decode` | 字符串 | 是 | | 另一个方法，恰好有一个 `string` 类型的参数，返回 `encode` 参数的类型 | 1 |
| `judge.round_trip.instances` | 字符串 | 是 | | `separate` 或 `same` | 1 |

- `round_trip` 只用于设计题，并且构造函数必须没有参数。
- 用例给出要编码的值，写成以 `encode` 的参数名为键的对象，而不是 `ops` 和 `args`；`expected` 是 `decode` 的结果（见[用例](#用例)）。
- 对每个用例，先构造一个或两个实例，用这个值调用 `encode`，再用 `encode` 返回的字符串调用 `decode`，并按 `judge.compare` 评判 `decode` 的结果。`encode` 返回的不是字符串时算运行错误。
- `separate`：`decode` 在第二个实例上运行，与 LeetCode 的 `ser = Codec(); deser = Codec(); deser.deserialize(ser.serialize(root))` 相同，所以解答无法借助实例状态传递这个值。`same`：两次调用在同一个实例上运行（535：`codec.decode(codec.encode(url))`）。
- 两个实例在同一个进程中。其他方法不会被调用，对编码本身的要求（449 的"尽可能紧凑"）不作评判。

内置示例题 0297（`separate`；0535 是 `same`）：

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

## 用例

`cases.jsonl` 每行一个用例，是一个只含以下字段的 JSON 对象：

| 字段 | 类型 | 必需 | 含义 | 起始版本 |
|---|---|---|---|---|
| `input` | 对象 | 是 | 用例的输入，按题目种类而定（见下文） | 1 |
| `expected` | JSON 值 | 否 | 期望输出，按题目种类而定（见下文）。`null` 也是一个值：`"expected": null` 是一个期望输出，缺少这个字段才表示没有 | 1 |
| `example` | 布尔值 | 否 | 该用例是题面中的示例 | 1 |
| `note` | 字符串 | 否 | 对用例的说明 | 1 |

- 一行恰好包含一个 JSON 值。空行，以及去掉开头空白后以 `//` 开头的行，会被忽略。
- 用例从 1 开始编号，只计入包含用例的行。
- 没有 `expected` 的用例是合法的，但无法评判（[判题结果](#判题结果)）。

按题目种类的 `input` 和 `expected`：

| 种类 | `input` | `expected` |
|---|---|---|
| 函数题 | 每个参数和隐藏输入各一个值，以名字为键，没有其他键。`ref` 参数取被引用节点的值，`copy` 参数不出现（[结构输入](#结构输入)） | 被评判的值：返回值、`judge.output` 的值（[原地修改的输出](#原地修改的输出)），或节点引用（`returns_ref`） |
| 设计题 | `{"ops": [...], "args": [...]}`，没有其他键。`ops` 是非空的名字数组：`ops[0]` 是类名（构造函数），其余是方法名。`args` 中每个操作对应一个数组，恰好包含该操作的参数 | 每个操作一项的数组：构造函数和 `void` 方法为 `null`，其余为返回值 |
| 编解码往返题 | 一个键，即 `encode` 的参数名，值为要编码的值 | `decode` 的结果 |

`judge.compare: any_of` 时，`expected` 改为由上述形式的所有可接受值组成的数组。`checker` 时，它是一个合法答案。各个值必须使用其声明类型的编码（[类型](#类型)）；`NextTreeNode` 的输出使用按层的形式。

内置示例题 0146：

```json
{"input": {"ops": ["LRUCache","put","put","get","put","get","put","get","get","get"], "args": [[2],[1,1],[2,2],[1],[3,3],[2],[4,4],[1],[3],[4]]}, "expected": [null,null,null,1,null,-1,null,-1,3,4], "example": true}
```

## 判题

评判一个用例，就是按 `judge.compare` 比较解答的输出与 `expected`，或者交给题目的判题程序。

### 比较方式

| `judge.compare` | 输出被接受的条件 | 起始版本 |
|---|---|---|
| `exact` | 与 `expected` 相等 | 1 |
| `unordered` | `expected` 和输出都是数组，且作为多重集合相等：忽略最外层数组的顺序 | 1 |
| `unordered_deep` | 同 `unordered`，并且每一层嵌套数组的顺序也都忽略 | 1 |
| `any_of` | 与 `expected`（所有可接受答案组成的数组）中的某一项相等 | 1 |
| `checker` | `checker.py` 接受它；`expected` 是传给判题程序的一个合法答案（[判题程序](#判题程序)） | 1 |

- 相等指 JSON 值相等：对象按键和值比较，数组逐个元素比较，字符串、布尔值和 `null` 须完全相同。两个数字都是整数（不带小数部分或指数，在有符号 64 位范围内）时，值相等才相等。其他数字对在容差 t（`judge.tolerance`）下满足 \|a − b\| ≤ t 或 \|a − b\| ≤ t · max(\|a\|, \|b\|) 时相等；t = 0 时，值相等才相等（`2` 等于 `2.0`）。
- 容差适用于所有比较方式。
- `unordered` 和 `unordered_deep` 要求函数题或编解码往返题被评判的值是数组类型。设计题被评判的值是结果数组，忽略的就是它的顺序。
- 不是编解码往返题的设计题不得使用 `any_of`：它们的 `expected` 每个操作一项。

### 原地修改的输出

用于修改输入的函数题：

- `judge.output: <输入>` 指向一个参数或隐藏输入。被评判的值是调用结束后它的值，按它的类型编码，而不是返回值。内置示例题 0189（`rotate`，返回 `void`）评判 `nums`。
- `judge.output_length: return` 只评判 `judge.output` 的前 k 个元素，k 是返回值（LeetCode 的"前 k 个元素之后的内容不重要"）。它要求有 `judge.output`、其类型为数组，并且返回类型为 `integer` 或 `long`。返回的 k 不在 0 到数组长度之间时算运行错误。

内置示例题 0027：

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

### 时限

`judge.timeout` 是解答在为它选定的用例上整次运行的墙钟时限，不是每个用例的时限。它用来防止卡死和死循环，不评判复杂度。到时后运行被终止，没有结果的用例判为超时（Time Limit Exceeded）；已经完成的用例保留各自的结果。判题程序的运行另有一个同样长度的时限。默认值是工作区的 `timeout`（未配置时为 10s）。

### 就绪

有 `judge.compare` 的题目是**就绪**的。`judge.compare` 没有默认值，即使是 `exact` 也必须写出。不就绪的题目是合法的，但它的判题规则尚未确定：它的用例不作评判（Not Judged），也不能提交。

### 判题结果

一次运行对每个用例给出以下结果之一：

| 结果 | 何时 |
|---|---|
| Compile Error | 解答无法构建：Go 编译错误、Python 语法错误。这是整次运行的结果，没有用例会运行 |
| Runtime Error | 用例出错（异常、panic、驱动程序无法解码的输入，或者编码无法表达的结果），或者程序在给出该用例的结果之前就退出了 |
| Time Limit Exceeded | 用例给出结果之前，[时限](#时限)已到 |
| No Expected Output | 用例运行了，但没有 `expected` |
| Not Judged | 用例运行了，但题目不是[就绪](#就绪)的 |
| Wrong Answer | 按 `judge.compare` 输出与 `expected` 不符，或者判题程序拒绝了它 |
| Accepted | 输出相符，或者判题程序接受了它 |

前三种结果在评判之前确定；No Expected Output 先于 Not Judged 确定。整次运行的结果是按用例顺序第一个结果为 Runtime Error、Time Limit Exceeded 或 Wrong Answer 的用例的结果。没有这样的用例时，只要有用例被接受（或者没有运行任何用例）就是 Accepted，否则有用例未评判时为 Not Judged，否则为 No Expected Output。

判题程序出错不是一种判题结果：见[判题程序](#判题程序)。

## 判题程序

`judge.compare: checker` 的题目必须在其目录中有 `checker.py`，其他比较方式（或没有比较方式）的题目不得有。不论解答用什么语言，判题程序都用 Python 编写，并且必须能在 Python 3.10 上运行。

```python
def check(input, output, expected):
    """Return None or True to accept; False or a message string to reject."""
```

`checker.py` 在模块顶层定义 `check`。它的参数是解码后的 JSON（dict、list、字符串、数字、布尔值、`None`）：

| 参数 | 函数题 | 设计题 | 编解码往返题 |
|---|---|---|---|
| `input` | 用例的 `input` 对象：按名字给出参数和隐藏输入（`ref` 是给出的值，没有 `copy` 参数） | `{"ops": [...], "args": [...]}` | `{<encode 的参数>: 值}` |
| `output` | 被评判的值，形式同 `expected` | 结果数组，每个操作一项 | `decode` 的结果 |
| `expected` | 用例的 `expected`，一个合法答案 | | |

- 返回值：`None` 或 `True` 表示接受；`False` 表示拒绝；字符串表示拒绝，并以该字符串作为信息。被拒绝的用例判为 Wrong Answer，并和它的信息一起显示。信息应当说清哪里错了：`"'bb' is not a substring of s"`。
- 对每个产生了输出并且有 `expected` 的用例，按用例运行的顺序调用一次 `check`，所有调用在解答运行结束后的同一个进程中进行，该进程的时限与[时限](#时限)长度相同。已经失败的用例（Runtime Error、Time Limit Exceeded）和没有 `expected` 的用例不会传入。
- 无法加载、没有可调用的 `check`、抛出异常、返回其他值或超时的判题程序，说明题目本身有问题，而不是对解答的判定：这次运行以错误结束，错误中注明 `checker.py` 和用例。
- 判题程序可以从 `hc_runtime` 导入公开名称（[Python](#python)）来重建结构，如 `decode("TreeNode", output)`；除此之外只能使用标准库。
- 判题程序必须接受每个正确的输出。对于随机题（LeetCode 384、528），输出要检查分布是否合理，这意味着正确的解答被拒绝的概率必须可以忽略不计。

内置示例题 0005（最长回文子串，`"bab"` 和 `"aba"` 都正确）：

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

## 解答 ABI

解答必须是什么样子，驱动程序才能调用它。用户的解答、参考解（`reference/solution.*`）和初始代码（`templates/solution.*`）都是解答。参考解和初始代码必须遵守本节，并且只使用标准库；初始代码必须保持签名，即这里规定的函数名、类名、方法名、参数顺序和类型。解答按 `function.params`（或构造函数和方法参数）的顺序收到参数，不含隐藏输入，绑定方式见[类型](#类型)。

### Python

解答文件是 `solution.py`。

| | 函数题 | 设计题 |
|---|---|---|
| 入口 | 一个 `Solution` 类，带有名为 `function.name` 的方法，每个用例无参数实例化一次；或者在模块没有定义 `Solution` 时，一个名为 `function.name` 的模块级函数 | 一个名为 `class.name` 的类，`__init__` 接收构造函数参数，方法名为 `class.methods` 中的名字 |

- 未捕获的异常算运行错误；语法错误算编译错误。
- 协议的类型来自 `hc_runtime` 模块。它的公开 API 正是其 `__all__` 中的名称：`ListNode`、`TreeNode`、`GraphNode`、`NaryNode`、`RandomListNode`、`NextTreeNode`、`QuadNode`、`MultilevelNode`、`NestedInteger`、`Employee`、`decode` 和 `encode`。解答和判题程序可以导入这些名称，不得从中导入其他任何东西。
- 题目的 `Node` 类型以 `Node` 这个名字导入：`from hc_runtime import GraphNode as Node`。其他类型保留原名：`from hc_runtime import ListNode, TreeNode`、`from hc_runtime import NestedInteger`。
- `decode(type, value)` 把一个 JSON 值转成协议类型 `type`（规范名称，如 `"TreeNode"` 或 `"integer[]"`）对应的 Python 值，`encode(type, value)` 做相反的转换。
- 参考解必须导入它用到的名称。初始代码可以省略导入：不包含 `hc_runtime` 的初始代码前面会加上标准的导入块（`typing` 中的名称、`ListNode`、`TreeNode`，以及题目的 `Node`、`NestedInteger` 和 `Employee`）。

### Go

解答文件是 `solution.go`，属于 `package main`。

| | 函数题 | 设计题 |
|---|---|---|
| 入口 | `func <function.name>(<参数>) <结果>`；`void` 时没有结果 | `type <class.name> struct`、`func Constructor(<构造函数参数>) <class.name>`（或 `*<class.name>`），以及 `class.methods` 中每一项对应的方法，接收者与初始代码一样是指针 |

- 方法名首字母大写（`getMin` 变成 `GetMin`），与 LeetCode 的 Go 初始代码一致；编解码往返题的 `encode` 和 `decode` 方法例外，保留原名（`serialize`）。
- 驱动程序在每道题中都预先定义 `ListNode` 和 `TreeNode`，在用到它们的题目中预先定义题目的 `Node` 类型、`NestedInteger` 和 `Employee`。解答不得声明它们。
- `main` 和以 `hc` 开头的包级标识符是保留的。
- 解答是一个没有依赖的独立模块，所以只能导入标准库。
- panic 算运行错误；构建失败算编译错误。
- 没有 `package` 子句的初始代码前面会加上 `package main`。

### 语言版本

| 语言 | 支持的最低版本 |
|---|---|
| Python | 3.10 |
| Go | 1.22 |

参考解、初始代码和判题程序必须能在这些版本上运行，包括语言和标准库：只有在受支持的工具链上，才会用参考解计算期望输出或检查参考解。用户的解答可以使用其工具链支持的任何特性。要按 Go 1.22 检查 Go 代码，在它的模块中写 `go 1.22` 并运行 `go vet`，其中的 `stdversion` 检查会报告较新的标准库符号。

## 合法性规则

合法的题目满足以下每一条规则。`hc validate` 检查关于 `problem.yaml`、`cases.jsonl` 和题库的规则；`hc validate --run` 还检查关于参考解的规则。

**problem.yaml**

1. 必须是合法的 YAML，不得包含不认识的字段。
2. `slug` 必须匹配 `^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`；`title` 不得为空；有 `difficulty` 时，它必须是 `easy`、`medium` 或 `hard`（大小写不限）。
3. `kind` 必须是 `function` 或 `class`，必须有对应的段，其中有 `name`，并且不得有另一个段。类必须至少有一个方法，方法名必须非空且互不相同，首字母大写后也互不相同。
4. 参数名必须非空且唯一：`function.params` 和 `function.hidden` 合在一起算，构造函数内，以及每个方法内。
5. 每个类型都必须是[受支持的类型](#类型)，并在数组限制之内；`void` 只能作返回类型。一道题不得使用一种以上的 `Node` 类型。
6. `cycle`、`join`、`ref` 和 `copy` 只能出现在函数参数上，每个参数最多一个，并且必须遵守[结构输入的规则](#结构输入)；`hidden` 和 `returns_ref` 也必须遵守。
7. `returns_copy` 必须遵守 [returns_copy 的规则](#returns_copy)。
8. `judge.output` 和 `judge.output_length` 只能出现在函数题中。有 `judge.output` 时，它必须指向一个不带 `cycle` 的参数或隐藏输入；`void` 函数必须有 `judge.output`。
9. 有 `judge.output_length` 时，它必须是 `return`，并且 `judge.output` 指向数组、返回类型为 `integer` 或 `long`。
10. `judge.round_trip` 只能出现在设计题中，并且必须遵守[编解码往返题的规则](#编解码往返题)。
11. 有 `judge.compare` 时，它必须是一种[比较方式](#比较方式)。使用 `unordered` 或 `unordered_deep` 时，函数题或编解码往返题被评判的值必须是数组。不是编解码往返题的设计题不得使用 `any_of`。
12. 有 `judge.tolerance` 时，它必须是有限的数，不小于 0。
13. `judge.compare: checker` 时必须有 `checker.py`，其他情况下不得有。
14. 有 `judge.timeout` 时，它必须是正的时长。

**cases.jsonl**

1. 每个非空、非注释的行都必须是一个 JSON 对象，包含 `input`（对象），可选 `expected`、`example`（布尔值）和 `note`（字符串），没有其他字段。
2. 函数题用例的 `input` 必须恰好以参数（`copy` 参数除外）和隐藏输入为键；设计题用例恰好有 `ops` 和 `args`；编解码往返题用例恰好有 `encode` 的参数。
3. 每个值都必须使用其类型的编码（[类型](#类型)），包括树和评测系统定义的类型的约束：`TreeNode` 的层序只为实际存在的节点列出子节点、邻居在图中、随机下标在链表内、`NaryNode`、`QuadNode` 和 `MultilevelNode` 的形式合法、员工 id 互不相同且下属都已知。`ref` 的值必须是整数。
4. 结构输入必须吻合：`cycle` 的位置在链表内，被接上的两个链表从 `skip` 和 `at` 开始的值相同，`ref` 的值恰好出现一次。
5. 在设计题用例中，`ops` 和 `args` 的长度必须相同且不为零，`ops[0]` 必须是类名，其余每个操作都必须是方法，`args` 的每一项都必须恰好包含该操作的参数，且类型相符。
6. 有 `expected` 时，它必须具有被评判的值的形式（[用例](#用例)）：`any_of` 时为这种值组成的数组；设计题用例中每个操作一项，构造函数和 `void` 方法为 `null`。

**题库**

1. 目录必须使用规范名称（[标识](#标识)）。
2. `slug` 在题库中必须唯一，有 `id` 时 `id` 也必须唯一。

**参考解、初始代码和判题程序**

1. 每份参考解都必须能构建，并且在受支持的工具链上，按题目的判题规则通过每个有 `expected` 的用例（Accepted；题目未就绪时为 Not Judged）；没有 `expected` 的用例必须能无错误地运行。有判题程序时，判题程序在任何参考解输出上都不得出错。
2. 参考解、初始代码和判题程序必须遵守[解答 ABI](#解答-abi) 和[判题程序](#判题程序)的规定，并且能在[支持的最低版本](#语言版本)上运行。`hc validate` 不检查这一条。
