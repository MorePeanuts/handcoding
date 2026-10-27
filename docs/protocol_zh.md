# handcoding 协议

[English](protocol.md) · 简体中文

本文定义两份协议：

1. **题目协议**：题库里的一道题在磁盘上如何组织。编写自己的题目（`hc new`、`hc import`）时参照它。
2. **驱动协议**：hc 与各语言测试驱动之间的约定。为 hc 增加新语言时参照它。

---

## 1. 题目协议

### 手写一道题

```sh
hc new sum-array --id 9001            # 生成 problems/9001-sum-array/（设计题加 --kind class）
# 编辑 problem.yaml 和 description.md，在 cases.jsonl 中写输入，写 reference/solution.py
hc cases build sum-array              # 运行参考解，算出 "expected"
hc validate sum-array --run           # 全面检查；每份参考解都必须通过每个用例
hc test sum-array                     # 之后就和其他题目一样练习
```

一道完整的函数题如下：

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

运行 `hc cases build sum-array` 后，三行用例会分别加上 `"expected": 6`、`-5` 和 `3000000000`。`expected` 也可以自己写，`hc validate --run` 都会用参考解核对一遍。

设计题把 `function` 换成 `class`，每个用例是一串操作：

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

### 目录结构

```
problems/0001-two-sum/          目录名：id 是数字时为 <4 位编号>-<slug>，否则为 <slug>
  problem.yaml                  元数据、签名和判题选项（必需）
  description.md                题面，Markdown 格式
  assets/rev1ex1.jpg            题面用到的图片（可选），在题面中写作 ![](assets/rev1ex1.jpg)
  cases.jsonl                   测试用例，每行一个 JSON 对象
  reference/solution.py         参考解（可选，任意语言），用于计算期望输出
  reference/solution.go
  templates/solution.go         初始代码（可选；缺省时根据签名生成）
  templates/solution.py
  checker.py                    判题程序，配合 judge.compare: checker 使用（见[判题程序](#判题程序)）
```

`hc import` 只复制以上这些文件和目录，其余内容（编辑器文件、`solutions/` 目录等）不会被导入。

### problem.yaml

| 字段 | 含义 |
|---|---|
| `id` | 题目编号（字符串，可选），用于排序和查找（`hc test 9001`）。在题库中必须唯一。是数字时也会作为目录名前缀 |
| `slug` | 唯一标识，短横线格式：字母、数字和单个短横线，建议全部小写（必需） |
| `title` | 标题（必需） |
| `difficulty` | `easy`、`medium` 或 `hard` |
| `tags` | 标签列表 |
| `kind` | `function`（单个函数）或 `class`（设计题）。缺省时，有 `class` 段则为 `class`，否则为 `function` |
| `function` | 函数题的签名：`name`、`params`（`{name, type}` 列表，每项最多带 `cycle`、`join`、`ref`、`copy` 之一）、`hidden`（不传给解答的输入）、`returns`（缺省为 `void`）、`returns_ref`、`returns_copy`。见[结构输入](#结构输入)和 [`returns_copy`](#returns_copy) |
| `class` | 设计题的签名：`name`、`constructor`（`{name, type}` 列表）、`methods`（`{name, params, returns}` 列表） |
| `judge.compare` | 输出的比较方式（见下文）。没有默认值：不写就表示题目*未就绪*（见下文） |
| `judge.tolerance` | 浮点误差。返回 `double` 时默认为 `1e-5`；绝对误差或相对误差满足其一即可 |
| `judge.output` | 原地修改类题目：调用结束后比较这个参数（或隐藏输入）的值，而不是返回值 |
| `judge.output_length` | `return`：只判 `judge.output` 的前 k 个元素，k 是函数返回的整数。用于 LeetCode 的"删除重复项"这类题 |
| `judge.round_trip` | 用于把一个值编码再解码回来的设计题：`encode`、`decode`（方法名）和 `instances`（`separate` 或 `same`）。见[编解码往返题](#编解码往返题) |
| `judge.time_limit` | 整次运行的时限，写成 `5s`、`500ms` 这样的时长。默认取 hc.yaml 中的 `timeout` |
| `source` | 题目来源：`site`、`url` |

不认识的字段会报错，所以 `judgee:`、`parms:` 这类拼写错误会被指出来，而不是悄悄退回默认值。

> **YAML 的坑。** 在紧凑的 `{...}` / `[...]` 写法里，`[]` 本身是 YAML 语法，所以数组类型必须加引号：`{name: nums, type: "integer[]"}`。用块状写法（像上面的例子那样把 `type: integer[]` 单独写一行）时，引号可加可不加。

**未就绪。** `judge.compare` 必须显式写出，即使是 `exact` 也要写。没写的题目，判题规则就是未确定的：`hc submit` 会拒绝提交，`hc test` 只把输出和期望输出并排列出，不给结论（显示 "Not Judged"），`hc validate` 会给出警告。`hc fetch` 拉取的题目有意处于这个状态，因为 LeetCode 不公开判题方式；由 `hc gen` 根据题面决定判题规则，或者由你手动设置。

原地修改类题目返回 `void`，并指明要判哪个参数：

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
  output: nums      # expected 是调用结束后 nums 的值
```

如果函数返回长度 k，并且只有前 k 个元素有意义（"前 k 个元素之后的内容不重要"）：

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
  output_length: return   # 判 nums[:k]，k 是返回值
```

比较方式：

| `compare` | 含义 | 例题 |
|---|---|---|
| `exact` | JSON 值相等（数字按数值比较） | 大多数题目 |
| `unordered` | 最外层数组视为多重集合，不计顺序 | Two Sum、全排列 |
| `unordered_deep` | 每一层数组都不计顺序 | 字母异位词分组、子集 |
| `any_of` | `expected` 是所有可接受答案组成的数组 | 合法答案不多的题目 |
| `checker` | 由题目目录下的 `checker.py` 判定；`expected` 是一个合法答案 | 合法答案很多（"返回任意一个"）、输出随机的题目 |

`unordered` 和 `unordered_deep` 要求结果是数组。

### 编解码往返题

有些设计题要求写一个编码器和一个解码器，编码格式由题解自己决定：二叉树的序列化与反序列化（297）、序列化和反序列化二叉搜索树（449）、TinyURL 的加密与解密（535）。能评判的只有 `decode(encode(x))`；编码得到的字符串既无法写进用例，也无法比较。`judge.round_trip` 用来描述这类题目：

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

用例给出的是要编码的值，键为编码器的参数名，不写 `ops` 和 `args`；`expected` 是解码器的结果，和平常一样由 `hc cases build` 用参考解填写。编解码器正确时，它就等于输入：

```json
{"input": {"root": [1,2,3,null,null,4,5]}, "expected": [1,2,3,null,null,4,5], "example": true}
```

对每个用例，驱动先解码输入，创建一个实例并用它调用 `encode`，再把 `encode` 返回的字符串传给 `decode`，然后按 `judge.compare`（`exact`，或[判题程序](#判题程序)）评判 `decode` 的结果。`instances` 没有默认值，因为它决定了题解能否在两次调用之间保留状态：

- `separate`：`decode` 在第二个实例上调用，和 LeetCode 的 `ser = Codec(); deser = Codec(); ans = deser.deserialize(ser.serialize(root))` 一样（297、449）。只把树记在实例上的编解码器会失败。
- `same`：两次调用在同一个实例上（535：`codec.decode(codec.encode(url))`），所以编解码器可以保存映射。

两个实例在同一个进程中，和 LeetCode 一样。`encode` 返回的不是字符串时判为运行错误。运行时间包括两次调用。类的其他方法不会被调用，449 的"尽可能紧凑"这类要求也不评判。

内置示例题 297（`separate`）和 535（`same`）分别展示了这两种情况。`hc fetch` 会根据 LeetCode 初始代码注释中的调用方式（`ans = deser.deserialize(ser.serialize(root))`，或经过一个变量的同样调用）为这类题目设置 `round_trip`，签名也取自初始代码；`judge.compare` 和平常一样留给 `hc gen` 决定。

规则：`round_trip` 只用于设计题；`encode` 和 `decode` 是类中两个不同的方法；`encode` 恰好有一个参数并返回 `string`；`decode` 恰好有一个 `string` 参数，返回 `encode` 参数的类型；构造函数没有参数；`instances` 是 `separate` 或 `same`。

### 判题程序

当一个用例的合法答案多到无法列举，或者输出是随机的，就交给程序判定：写 `judge.compare: checker`，并在 `problem.yaml` 旁边放一个 `checker.py`。不论题解用什么语言，判题程序都用 Python 编写，所以用判题程序评测的题目即使用 Go 作答也需要 Python 3.10 或更新版本；hc 管理的虚拟环境（`.hc/venv`）会提供它。

```python
# checker.py
def check(input, output, expected):
    """Return None or True to accept; False or a message string to reject."""
```

- `input` 是解码后的用例输入：函数题是参数（及隐藏输入）组成的对象，设计题是 `{"ops": [...], "args": [...]}`，[编解码往返题](#编解码往返题)是编码器的参数。
- `output` 是解码后的题解输出，形式与 `expected` 相同（原地修改的题目是 `judge.output` 的值，设计题是每个操作的结果，编解码往返题是解码器的结果）。
- `expected` 是用例的期望输出，和其他题目一样由参考解通过 `hc cases build` 计算。它是一个合法答案，所以判题程序可以比较一个派生值，而不必重新求解：最长回文的长度、子集的大小。没有 `expected` 的用例不评判（"No Expected Output"）。
- 需要重建结构时，导入题解使用的编解码函数和类型：`from hc_runtime import decode, TreeNode`，再 `decode("TreeNode", output)`。
- 被拒绝的用例判为 Wrong Answer，判题程序给出的信息会和它一起显示在 `hc test`、`hc submit`、TUI 和提交历史中。信息要说清哪里错了：`"[1,2] is not a permutation of nums"`、`"length 2, but the longest palindrome has length 3"`。

最长回文子串（5），`"bab"` 和 `"aba"` 都正确：

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

每次运行时，hc 在题解运行完后启动一次判题程序：一个辅助脚本对每个产生了输出的用例调用 `check`（已经判为运行错误或超时的用例不会送去）。判题程序的时间限制与题解相同（`judge.time_limit`）。判题程序无法加载、抛出异常、返回 `None`、布尔值和字符串以外的值，或者超时，都说明题目本身有问题，而不是答案错误：这次运行以错误结束，错误中注明 `checker.py`、用例和调用栈，`hc submit` 也不会记录提交。判题程序打印的内容只在这时显示。

**随机题**（打乱数组、按权重随机选择）是设计题，每个用例包含大量调用，从几百到几千次。判题程序既检查每个输出是否合法，也检查它们的分布是否合理：把观察到的频率和期望频率比较，界限要足够宽松，让正确的题解几乎不可能失败（在判题程序中写明误判概率，例如每个用例低于 1e-6），同时明显不随机的题解（总是选同一个、原样返回数组）会失败。

内置示例题 5、108、210、324（原地修改）、384 和 528（随机）展示了各类判题程序。

### 类型

类型名沿用 LeetCode `metaData` 中的名字，所以从 LeetCode 获取的题目无需转换：

| 类型 | JSON | Go | Python |
|---|---|---|---|
| `integer` / `long` | 整数 | `int` / `int64` | `int` |
| `double` | 数字 | `float64` | `float` |
| `boolean` | `true` / `false` | `bool` | `bool` |
| `string` | 字符串 | `string` | `str` |
| `character` | 单字符字符串 `"a"` | `byte` | `str` |
| `ListNode` | 数组 `[1,2,3]` | `*ListNode` | `ListNode \| None` |
| `TreeNode` | 层序数组 `[3,9,20,null,null,15,7]` | `*TreeNode` | `TreeNode \| None` |
| `GraphNode`、`NaryNode`、`RandomListNode`、`NextTreeNode`、`QuadNode`、`MultilevelNode` | 见 [LeetCode 的 Node 类型](#leetcode-的-node-类型) | `*Node` | `Node \| None` |
| `NestedInteger` | 整数或（嵌套）数组，`[1,[4,[6]]]`；见[下文](#nestedinteger-和-employee) | `*NestedInteger` | `NestedInteger` |
| `Employee` | `[id, importance, [下属 id]]` | `*Employee` | `Employee` |
| `T[]` | 数组，最多 3 层；`ListNode`、`TreeNode`、`NestedInteger`、`Employee` 的数组只支持 1 层，`Node` 类型不能组成数组 | `[]T` | `list[T]` |
| `void` | 只能作返回类型 | — | `None` |

`list<T>` 会规范化为 `T[]`；`int`、`bool`、`char`、`float` 会规范化为 `integer`、`boolean`、`character`、`double`。

### LeetCode 的 Node 类型

LeetCode 把六种不同的结构都叫作 `Node`。hc 给每种结构各起一个类型名；在解答代码里它们仍然都是名为 `Node` 的类型（Go）或类（Python），字段名与 LeetCode 一致，所以一道题只能用其中一种。它们不能组成数组，[结构输入](#结构输入)也只支持 `ListNode` 和 `TreeNode`。

| 类型 | 字段（Go；Python 用小写开头的同名字段） | JSON（用例输入和期望输出） | 题目 |
|---|---|---|---|
| `GraphNode` | `Val int`、`Neighbors []*Node` | 邻接表：第 i 项列出节点 i+1 的邻居，节点 i+1 的值就是 i+1。参数是节点 1。`[[2,4],[1,3],[2,4],[1,3]]`；`[[]]` 是单个节点，`[]` 表示没有节点 | 133 |
| `NaryNode` | `Val int`、`Children []*Node` | 层序：根、`null`，然后按层序依次写出每个节点的子节点，每组后面跟一个 `null`（去掉末尾的 `null`）：`[1,null,3,2,4,null,5,6]` | 429、559、589、590 |
| `RandomListNode` | `Val int`、`Next *Node`、`Random *Node` | `[[val, randomIndex], ...]`，`randomIndex` 是随机指针所指节点的下标（从 0 开始），或 `null`：`[[7,null],[13,0],[11,4],[10,2],[1,0]]` | 138 |
| `NextTreeNode` | `Val int`、`Left *Node`、`Right *Node`、`Next *Node` | 输入：和 `TreeNode` 一样的层序，所有 next 指针都为空。输出：从每层最左边的节点出发沿 next 指针读出的各层，`[[1],[2,3],[4,5,7]]`（LeetCode 写作 `[1,#,2,3,#,4,5,7,#]`，这不是 JSON） | 116、117 |
| `QuadNode` | `Val bool`、`IsLeaf bool`、`TopLeft`、`TopRight`、`BottomLeft`、`BottomRight *Node`（Python：`val`、`isLeaf`、`topLeft`……） | 层序，每个节点都有四个子节点位置（没有时为 `null`），每个节点写成 `[isLeaf, val]`，取 0 或 1，去掉末尾的 `null`：`[[0,1],[1,0],[1,1],[1,1],[1,0]]` | 427、558 |
| `MultilevelNode` | `Val int`、`Prev *Node`、`Next *Node`、`Child *Node` | LeetCode 的序列化：各层依次写出，每层以 `null` 结束；子层前面再加若干个 `null`，个数等于其父节点在上一层中的下标（从 0 开始）：`[1,2,3,4,5,6,null,null,null,7,8,9,10,null,null,11,12]` | 430 |

- Go 中，题目用到哪种 `Node`，驱动程序就定义哪种，不要自己声明。Python 中从 `hc_runtime` 以 `Node` 这个名字导入，初始代码已经写好：`from hc_runtime import GraphNode as Node`。Python 构造函数的参数与 LeetCode 一致，且都可省略：`Node(val, neighbors)`、`Node(val, children)`、`Node(x, next, random)`、`Node(val, left, right, next)`、`Node(val, isLeaf, topLeft, topRight, bottomLeft, bottomRight)`、`Node(val, prev, next, child)`。
- 解答可以自己创建节点（克隆题就需要）。返回的结构按同样的 JSON 形式输出，以下情况算运行错误：`GraphNode` 的结果不是一个值为 1 到 n 的图中的节点 1；随机指针指向返回链表之外的节点；`MultilevelNode` 的 `prev` 没有指向前一个节点（或者某层第一个节点的 `prev` 不为空），以及同一层有两个节点带子链表，这种结构无法序列化。
- 非叶子的 `QuadNode` 取哪个值都可以，所以输出时一律写成 `[0,1]`。

### NestedInteger 和 Employee

另外两个类型也由评测系统而不是解答定义，沿用 LeetCode 的名字：

- `NestedInteger` 存放一个整数，或一个由 `NestedInteger` 组成的列表。它的 JSON 形式是整数或由这类值组成的数组；`NestedInteger[]` 是它们组成的数组：`[[1,1],2,[1,1]]`（扁平化嵌套列表迭代器，341）。hc 实现了 LeetCode 的接口，所以解答也可以自己构造一个并返回（迷你语法分析器，385）。Go：`IsInteger() bool`、`GetInteger() int`、`SetInteger(int)`、`Add(NestedInteger)`、`GetList() []*NestedInteger`；零值是空列表。Python：`NestedInteger(value=None)`（不给值就是空列表）、`isInteger()`、`getInteger()`、`setInteger(value)`、`add(elem)`、`getList()`。
- `Employee` 的字段是 `Id`、`Importance`、`Subordinates []int`（Python：`id`、`importance`、`subordinates`），写作 `[id, importance, [下属 id]]`。`Employee[]` 中的 id 必须互不相同，每个下属都必须在其中：`[[1,5,[2,3]],[2,3,[]],[3,3,[]]]`（员工的重要性，690）。

Go 中，题目用到它们时驱动程序会定义。Python 中从 `hc_runtime` 导入：`from hc_runtime import NestedInteger`。

#### `returns_copy`

`function` 中的 `returns_copy: <参数>` 表示函数返回某个参数的深拷贝（克隆图、随机链表的复制）。如果返回的结构与该参数的结构共享任何节点，该用例算运行错误。这个参数的类型必须与返回类型相同，并且是 `ListNode`、`TreeNode` 或某种 `Node` 类型；`returns_copy` 不能与 `returns_ref` 或 `judge.output` 同时使用。

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

### 结构输入

有些题目传给解答的链式结构无法用普通 JSON 值描述：尾部连回自身的链表、共享尾部的两个链表、树中的某个节点。函数题用下面这些属性描述它们，和 LeetCode 评测系统处理自己的测试数据的方式一致：

| 属性 | 位置 | 含义 |
|---|---|---|
| `hidden` | `function` | 额外的用例输入（`{name, type}`），驱动程序会用到，但不传给解答 |
| `cycle: <输入>` | `ListNode` 参数 | 链表尾部连到下标为 `<输入>` 的节点（`<输入>` 是整数隐藏输入，从 0 开始）；`-1` 表示无环 |
| `join: {list, skip, at}` | `ListNode` 参数 | 链表保留自己的前 `skip` 个节点，然后接上参数 `list` 从下标 `at` 开始的节点（共享节点，不是副本） |
| `ref: <输入>` | `ListNode` / `TreeNode` 参数 | 用例给出节点的**值**；解答拿到结构 `<输入>` 中持有该值的节点 |
| `copy: <参数>` | `ListNode` / `TreeNode` 参数 | 不出现在用例中；解答拿到 `<参数>` 的深拷贝 |
| `returns_ref: <输入>` | `function` | 函数返回结构 `<输入>` 中的一个节点（或空），按节点本身判题 |

一个参数最多带 `cycle`、`join`、`ref`、`copy` 中的一个。初始代码只列出参数，类型是节点类型（`head *ListNode`、`p: TreeNode | None`）。

**`cycle`**，环形链表 II。用例给出 `head` 和 `pos`，解答只拿到 `head`：

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

**`join`**，相交链表。`headA` 和 `headB` 是完整的链表，包括共享的尾部；`skipA` 和 `skipB` 是共享部分之前的节点数。`skipA` 等于 `headA` 的长度时两个链表不相交，此时 `skipB` 等于 `headB` 的长度：

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

**`ref`**，二叉树的最近公共祖先。`p` 和 `q` 在用例中是值，在解答中是 `root` 的节点。被引用的值在结构中必须恰好出现一次：

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

`ref` 可以指向隐藏输入，`judge.output` 也可以是隐藏输入。删除链表中的节点只把 `node` 传给解答，判的是整个链表：

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

**`copy`**，找出克隆二叉树中的相同节点。`cloned` 不出现在用例中：

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

**`returns_ref`** 对返回的节点编码，使判题看的是节点本身，而不是它后面的值：`ListNode` 结构中编码为从 0 开始的**下标**（链表的值可能重复），`TreeNode` 结构中编码为节点的**值**（引用树节点的题目保证值互不相同），没有节点时为 `null`。返回不属于该结构的节点是运行错误。`expected` 使用同样的编码。

规则：`cycle`、`skip`、`at` 指向整数隐藏输入；`join.list` 指向另一个普通的 `ListNode` 参数；`ref` 和 `returns_ref` 指向同一节点类型、持有整个结构（不是 `ref`）的参数或隐藏输入；`copy` 指向同类型的普通参数；每个隐藏输入都必须被用到；`judge.output` 不能是带环的链表。设计题不支持这些属性。

### cases.jsonl

每行一个用例：

```json
{"input": {"nums": [2,7,11,15], "target": 9}, "expected": [0,1], "example": true}
{"input": {"nums": [-1,-2,-3,-4,-5], "target": -8}, "expected": [2,4], "note": "negative numbers"}
```

- `input`：函数题是一个对象，每个参数和隐藏输入恰好对应一个值，键为名字。`ref` 参数给出被引用节点的值；`copy` 参数不写。设计题是 `{"ops": [...], "args": [[...], ...]}`；`ops[0]` 必须是类名（即构造函数），`args` 的每一项恰好是对应操作的参数。[编解码往返题](#编解码往返题)是编码器的参数：`{"root": [1,2,3]}`。
- `expected`：期望输出。设计题是每个操作返回值组成的数组，构造函数和 `void` 方法写 `null`；编解码往返题是解码器的结果。`any_of` 模式下是可接受答案组成的数组，需要手写：参考解只能给出一个答案，所以 `hc cases build` 拒绝填写。`checker` 模式下是一个合法答案，会传给判题程序。其他模式下缺省时由 `hc cases build` 用参考解计算。
- `example`：该用例是题面中的示例。`hc test` 默认只运行这些用例（一个都没标时运行前 3 个）。
- `note`：用例说明。
- 不允许出现其他字段。空行和以 `//` 开头的行会被忽略。用例从 1 开始编号，只计算真正包含用例的行。

### 参考解和初始代码

参考解的写法和用户的解答完全一样，放在 `reference/solution.py` 或 `reference/solution.go`。用 `hc start` 生成的初始代码就是去掉了函数体的参考解，可以照着写。

| | 函数题 | 设计题 |
|---|---|---|
| Python | `class Solution`，其中有名为 `function.name` 的方法；也可以是同名的模块级函数 | 名为 `class.name` 的类；`__init__` 接收构造函数参数；方法名与 `methods` 中一致 |
| Go | `package main`；`func <function.name>(...)`，参数类型见上面的类型表 | `type <Name> struct`、`func Constructor(...) <Name>`，以及指针接收者的方法，方法名首字母大写（`getMin` → `GetMin`）；编解码往返题的编码器和解码器例外，和 LeetCode 的初始代码一样保留原名（`serialize`） |

- `ListNode` 和 `TreeNode` 已经提供：Python 中用 `from hc_runtime import ListNode, TreeNode` 导入；Go 中已预先定义（字段为 `Val`、`Next`、`Left`、`Right`），不要自己再声明。题目用到的 [`Node`](#leetcode-的-node-类型)、`NestedInteger` 和 `Employee` 也一样：Python 中写 `from hc_runtime import GraphNode as Node` 或 `from hc_runtime import NestedInteger`，Go 中已预先定义。
- 两份参考解都存在时，`hc cases build` 先用 Go，再用 Python；可以用 `-l python` 指定。`hc validate --run` 会运行每一份参考解。
- `templates/solution.go` 和 `templates/solution.py` 会替换自动生成的初始代码。它们必须保持签名不变，因为驱动要按签名调用。

### 语言版本

| 语言 | 最低支持版本 |
|---|---|
| Python | 3.10 |
| Go | 1.22 |

这些版本约束 hc 生成的代码（驱动、初始代码）和题目自带的代码：参考解和模板必须能在这些版本上运行。你自己的解答可以使用本机工具链支持的任何特性。

- `hc test`、`hc submit` 和 `hc debug` 会显示实际使用的工具链（`[Python 3.12.9]`），版本过低时给出警告，但仍然运行。
- `hc cases build` 和 `hc validate --run` 拒绝在版本过低的工具链上运行参考解，保证期望输出总是来自受支持的版本。
- 判题程序总是在 Python 3.10 或更新版本上运行，Go 题解也一样。
- LeetCode 模板中的 `X | None` 注解需要 Python 3.10。hc 在 `.hc/venv` 中用 Python 3.10 或更新版本创建虚拟环境来运行 Python（借助 uv，必要时自动下载），所以系统自带的 `python3` 版本较低（macOS 自带 3.9）也没有关系。在 `hc.yaml` 中设置 `python.interpreter` 可以改用你指定的解释器。
- Go 1.22 同时限制语言特性和标准库。要按这个版本检查 Go 代码，在它的 `go.mod` 中写 `go 1.22` 并运行 `go vet`，其中的 `stdversion` 检查会报告更新的标准库符号。

### 校验

```
hc validate <slug>          # 按协议检查
hc validate <slug> --run    # 同时在每个用例上运行每一份参考解，必须全部通过
```

`hc validate` 会报告以下错误，并指出用例编号和字段：

- `problem.yaml`：语法错误（带行号和列号）、不认识的字段、不合法的 slug、缺少标题、不支持的类型、`judge.output` 不是参数或隐藏输入、`void` 函数没写 `judge.output`、结构输入用法不对（`hidden`、`cycle`、`join`、`ref`、`copy`、`returns_ref`）、`returns_copy` 用法不对、用了多种 `Node` 类型、`judge.output_length` 用法不对、`judge.round_trip` 用法不对、不认识的 `judge.compare`、`judge.compare: checker` 却没有 `checker.py` 以及有 `checker.py` 却用了其他模式、不合法的 `judge.time_limit`；
- `cases.jsonl`：JSON 不合法、不认识的字段、缺少或多出的输入、值与声明的类型或评测系统定义的类型的形式不符（`input "nums": [1]: must be integer, got string "x"`；邻居不是图中的节点、随机下标超出链表、`null` 的位置不对、下属不存在）、结构输入对不上（环的位置超出链表、相交链表的尾部不同、被引用的值不存在或重复）、设计题中参数个数不对或方法不存在、编解码往返题写了 `ops` 和 `args`、`expected` 的形状不对；
- 加上 `--run` 时：参考解无法编译、运行出错，或与 `expected` 不一致（会同时显示两个值，以及判题程序的信息）；`checker` 模式下每个参考解输出都经过判题程序，判题程序出错也算错误。

以下情况会给出警告：判题规则未确定（没有 `judge.compare`）、有用例缺少 `expected`、没有标记示例用例、没有参考解、题面引用的图片不存在、编号与其他题目重复。

---

## 2. 驱动协议（增加新语言）

一门语言需要实现 `internal/lang.Language` 接口，并注册到 `internal/app.Languages`：

```go
type Language interface {
    ID() string                       // "go"；用于路径和 --lang
    Name() string
    Aliases() []string
    SolutionFile() string             // 用户编辑的文件，如 solution.go
    Starter(p *problem.Problem) (string, error)            // 初始代码
    Prepare(dir string, p *problem.Problem) error          // 写入驱动和运行时文件（每次运行前调用，必须幂等）
    Build(ctx context.Context, dir, binDir string) error   // 编译或检查语法；失败时返回 *lang.CompileError
    Command(ctx context.Context, dir, binDir string) *exec.Cmd
    DebugCommand(dir, binDir string, p *problem.Problem, opts lang.DebugOptions) (*exec.Cmd, error)
    Toolchain() (lang.Toolchain, error) // 检测编译器或解释器及其版本；找不到时返回错误
    Setup(ctx context.Context, w io.Writer) error // 准备 hc 管理的环境，如 Python 的 .hc/venv；运行前调用
}
```

驱动只负责**解码输入、调用用户代码、编码输出**，从不判题。所有判题都在 Go 中完成（`internal/judge`），所以新语言不需要重新实现比较逻辑。解码包括构建[结构输入](#结构输入)和 [`Node` 结构](#leetcode-的-node-类型)，编码包括 `returns_ref` 和 `returns_copy` 检查；与结构对不上的输入报告为以 `invalid input:` 开头的错误。

- `HC_CASES` 指向一个 JSON 文件，内容为 `[{"id": 1, "input": {...}}, ...]`。
- `HC_OUT` 指向一个文件，驱动以 JSON Lines 格式向其追加结果，每个用例一行：

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  `error` 非空表示运行时错误；尽量只保留用户代码中的调用栈。`elapsed_ns` 只统计用户代码的耗时。
- 用户的 stdout 和 stderr 直接输出到终端，所以可以用打印调试。
- 如果驱动崩溃或超时，没有结果行的用例分别判为 Runtime Error 或 Time Limit Exceeded。
- [判题程序](#判题程序)同样不归驱动管：它们由 Python 为所有语言运行（`lang.Checker`），再由 `internal/judge` 把结果变成判定。

现有的驱动可以作为参考：`internal/lang/golang`（生成代码加静态运行时）和 `internal/lang/python`（动态运行时，签名嵌入其中）。
