# handcoding 题目协议

[English](protocol.md) · 简体中文

> 本文是 [docs/protocol.md](protocol.md) 的译文。英文原文是规范文本，两者不一致时以英文为准。`hc docs protocol <id>` 打印英文原文的一节，节的 id 来自英文标题。

## 范围与版本

本文规定 handcoding 的题目协议：题目如何汇集成题目包，题目在磁盘上如何组织，各个文件的含义，用例如何评判，以及题目附带的代码（参考解、初始代码、判题程序）可以依赖什么。本文是规范性的。

MUST、MUST NOT、SHOULD、MAY 按 RFC 2119 的含义使用，译文中分别写作"必须"、"不得"、"应当"、"可以"。违反"必须"的题目是不合法的；[合法性规则](#合法性规则)汇总了所有这类规则。

这是**协议 1**。下文的每个字段、类型和模式，除非另有说明，都属于协议 1。题目包在清单中用 `protocol: N` 声明它所依据的协议（见 [pack.yaml](#packyaml)）；更新的协议可能增加特性，旧的题目包不会用到它们。

hc 自身的行为（`hc test` 运行哪些用例、`hc fetch` 如何导入题目、工作区把题目包放在哪里）记录在 `docs/how-to-use.md` 中，不在本文。hc 与各语言驱动程序之间如何通信是 hc 的内部细节，记录在 `CONTRIBUTING.md` 中。

### 协议版本

协议版本是整数，与 hc 的发布版本相互独立：不改变协议的 hc 版本，也不会改变题目包可以包含什么。

一个版本在定稿之前是草案；第一个实现它的 hc 正式版（不带 `-alpha.1` 这类预发布后缀的版本）发布时，它就定稿了，此后不再改变。版本还是草案时，hc 的预发布版本可以往里增加特性，也可以修改此前增加的特性，而不产生新版本：两个预发布版本可能读不了对方的题目包，较旧的预发布版本遇到题目包的新特性时可能直接拒绝，而不会提示你升级。需要长期可用的题目包应当声明已定稿的版本，并用 hc 正式版检查。下面的规则针对的是已定稿的版本。

- 只增加特性（一种类型、一种比较方式、一个字段）的版本，让依据更早版本编写的每个题目包保持合法，含义不变。本文为每个特性标明引入它的版本（"起始版本"列）。
- 修改或删除特性的版本是不兼容的：从这个版本起，工具不再读取声明更早版本的题目包，这些题目包需要更新。
- 题目包不得使用晚于其声明版本引入的特性，这样读取该版本的每个工具都能读取这个题目包。
- 工具读取的版本，从它仍支持的最早版本到它实现的最新版本。对于超出这一范围的题目包，工具必须在读取其他任何内容之前拒绝它，并说明该怎么做：题目包较新时升级工具，较旧时更新题目包。hc 的提示是 "this pack needs protocol 2, hc v0.1.0 supports up to 1, upgrade hc" 或 "this pack uses protocol 1, which hc v0.5.0 no longer reads, update the pack"。

| 版本 | 定稿于 | 变更 |
|---|---|---|
| 1 | hc v0.1.0（尚未发布：仍是草案） | 最初的协议：题目包和 `pack.yaml`、题目目录、`problem.yaml`、类型、签名、用例、判题、判题程序和解答 ABI，解答语言为 Go 和 Python。 |

### 不在范围内

协议 1 不定义以下内容，它们都将以后续的增量变更加入：

- 交互题，即解答要与评测程序对话的题目（issue #7）；
- `tests` 题，用测试代码而不是用例评判（issue #31）；
- 内存评判，以及[时间限制](#时间限制)以外的时间评判；
- Go 和 Python 以外的解答语言。

## 题目包

题目包是一起发布的一组题目，通常是一个独立的仓库。它是安装和更新的单位，也是题目[标识](#标识)的范围。一个题目包是一个目录，包含：

```
pack.yaml          清单；见 pack.yaml
problems/<dir>/    每道题目一个题目目录；见题目目录
...                其他内容都不属于题目包
```

- `pack.yaml` 必须位于题目包的根目录。
- `problems/` 下直接包含 `problem.yaml` 的每个目录都是题目包中的一道题目，目录名必须是该题目的规范名称（[标识](#标识)）。`problems/` 中的其他条目不属于题目包。没有 `problems/`、或 `problems/` 为空的题目包没有题目，它是合法的。
- 题目包中的其他内容（README、许可证、CI 配置、给智能体的说明和技能）都不属于题目包，工具会忽略它们。题目放在 `problems/` 下，这些文件就不会被当成题目。

### pack.yaml

`pack.yaml` 是一个 YAML 映射。不认识的字段在任何层级都是错误。

| 字段 | 类型 | 必需 | 默认值 | 约束 | 起始版本 |
|---|---|---|---|---|---|
| `protocol` | 整数 | 是 | | 题目包所依据的[协议版本](#协议版本)：正整数，不能是字符串或小数 | 1 |
| `name` | 字符串 | 是 | | 小写字母、数字和单个短横线：`^[a-z0-9]+(-[a-z0-9]+)*$`。不能是 `local`。题目包的标识（见下文） | 1 |
| `title` | 字符串 | 否 | `name` | 显示名称，使用默认语言区域 | 1 |
| `titles` | 映射 | 否 | `{}` | 其他语言区域的显示名称，以语言区域为键；见[语言区域](#语言区域) | 1 |
| `version` | 字符串 | 否 | 无 | 不带 `v` 前缀的 SemVer 2.0.0 版本号：`0.3.0`、`1.0.0-alpha.1`。仅供参考：安装时固定的是提交，而不是这个字段 | 1 |
| `description` | 字符串 | 否 | 无 | 仅供参考 | 1 |
| `authors` | 字符串列表 | 否 | `[]` | 仅供参考 | 1 |
| `license` | 字符串 | 否 | 无 | 应当是 SPDX 许可证表达式，如 `MIT` | 1 |
| `homepage` | 字符串 | 否 | 无 | 应当是 URL | 1 |
| `languages` | 字符串列表 | 否 | `protocol` 的所有语言 | 非空；所声明协议的语言 id，不得重复；见下文 | 1 |
| `locales` | 字符串列表 | 否 | 无：一种不具名的语言 | 非空；BCP 47 语言标签，不得重复。第一个是默认语言区域。见[语言区域](#语言区域) | 1 |
| `groups` | 分组列表 | 否 | `[]` | 见[分组](#分组) | 1 |
| `groups[].name` | 字符串 | 是 | | `^[a-z0-9]+(-[a-z0-9]+)*$`，在题目包的分组中唯一 | 1 |
| `groups[].title` | 字符串 | 否 | 分组的 `name` | 显示名称，使用默认语言区域 | 1 |
| `groups[].titles` | 映射 | 否 | `{}` | 其他语言区域的显示名称，以语言区域为键 | 1 |
| `groups[].problems` | 字符串列表 | 否 | `[]` | 题目包中题目的 slug，不得重复 | 1 |

- 最先读取的是 `protocol`。不读取所声明版本的工具会拒绝这个题目包，不再读取其余内容（[协议版本](#协议版本)），因为更新的协议可能增加了它不认识的字段。
- `name` 是题目包的标识。已安装的题目包、题解和提交历史都以它为键，所以修改题目包的名称是破坏性变更，就像修改题目的 slug 一样。`local` 保留给工作区中不属于任何题目包的题目。
- `languages` 列出题目包中的题目可以用哪些语言解答。不支持用其他语言解答，工具可以拒绝开始这样的解答。省略它表示所声明协议的所有语言，所以只有声明了更新的协议，题目包才会获得该协议新增的语言。参考解可以用协议的任何语言编写，不受 `languages` 限制。解答只在题目有参考解的语言中评判（[就绪](#就绪)），所以题目应当对 `languages` 中的每种语言都有参考解。判题程序总是用 Python 编写（[判题程序](#判题程序)）。

语言 id：

| Id | 语言 | 起始版本 |
|---|---|---|
| `go` | Go | 1 |
| `python` | Python | 1 |

包含所有字段的 pack.yaml：

```yaml
protocol: 1
name: leetcode
title: LeetCode
titles: {zh-CN: 力扣}
version: 0.3.0
description: Classic LeetCode problems with Go and Python references.
authors: [MorePeanuts]
license: MIT
homepage: https://github.com/MorePeanuts/handcoding-leetcode
languages: [go, python]
locales: [en, zh-CN]
groups:
  - name: arrays
    title: Arrays
    titles: {zh-CN: 数组}
    problems: [two-sum, rotate-array]
```

### 分组

分组把题目包中的题目编排成有名称的列表，如专题或学习计划。

- `groups` 是有序的，每个分组的 `problems` 也是：工具按这个顺序显示它们。
- 分组用 `slug` 引用题目，而不是用 id 或目录名，每一项都必须是题目包中某道题目的 slug。一个分组不得重复列出同一道题目。
- 一道题目可以属于多个分组，也可以不属于任何分组。分组可以为空。

### 语言区域

题目包可以用多种语言提供题面。这里的“语言”指题面的语言，不是解答语言（`languages`）。

- `locales` 用 BCP 47 语言标签列出这些语言，如 `en` 或 `zh-CN`，每个只出现一次（比较标签时不区分大小写）。题面文件名和 `titles` 的键必须与 `locales` 中的写法完全一致。第一个是题目包的**默认语言区域**。题目包有哪些语言区域、哪个是默认的，由作者决定。
- `description.md` 和 `title` 使用默认语言区域。其他语言区域的题面是 `description.<locale>.md`，如 `description.zh-CN.md`，标题是 `problem.yaml` 中的 `titles.<locale>`：`titles: {zh-CN: 两数之和}`。题目包和每个分组的 `titles` 用法相同，且是可选的。
- 声明了某个语言区域的题目包，必须为每道题目提供该语言区域的题面和标题：默认语言区域是 `description.md`，其他语言区域是 `description.<locale>.md` 和 `titles.<locale>`。
- `description.<locale>.md` 文件和 `titles` 的键必须是题目包声明的、默认语言区域以外的语言区域，默认语言区域的题面和标题是 `description.md` 和 `title`。
- 没有 `locales` 时，题面使用一种不具名的语言：只有 `description.md` 和 `title`，没有 `description.<locale>.md`，也没有 `titles`。
- 所有题面共用同一个 `assets/`。签名、用例、初始代码、参考解和判题程序都与语言区域无关。

## 题目目录

一道题目是一个目录，包含以下条目：

| 条目 | 必需 | 内容 |
|---|---|---|
| `problem.yaml` | 是 | 元数据、签名和判题选项；见 [problem.yaml](#problemyaml) |
| `description.md` | 题目包有 `locales` 时 | 题面，Markdown 格式，使用题目包的默认语言区域 |
| `description.<locale>.md` | 题目包的每个其他语言区域 | 该语言区域的题面；见[语言区域](#语言区域) |
| `assets/` | 否 | 题面引用的文件，如图片 |
| `cases.jsonl` | 否 | 测试用例；见[用例](#用例) |
| `reference/solution.py`、`reference/solution.go` | 否 | 参考解，可以只有其中一部分语言；解答只在有参考解的语言中评判（[就绪](#就绪)） |
| `templates/solution.py`、`templates/solution.go` | 否 | 初始代码，取代根据签名生成的初始代码 |
| `checker.py` | 仅限 `judge.compare: checker` | 判题程序；见[判题程序](#判题程序) |
| `generator.py` | 仅限有生成用例时 | 生成用例的生成器；见[生成器](#生成器) |

- 在没有 `locales` 的题目包中，没有 `description.md` 的题目题面为空。没有 `cases.jsonl` 的题目没有用例。两者都合法，但这样的题目没法真正拿来练习。
- 题面用相对于题目目录的路径引用本地文件，这些文件必须位于 `assets/` 下：`![](assets/rev1ex1.jpg)`。
- 其他条目不属于题目。工具可以忽略或丢弃它们。
- 参考解、初始代码、判题程序和生成器可以包含什么，由[解答 ABI](#解答-abi)、[判题程序](#判题程序)和[生成器](#生成器)规定。

### 标识

- `slug` 是题目在所属题目包中的标识，在题目包中必须唯一；不同题目包中的题目可以有相同的 slug。题解和提交历史以题目包的名称和 slug 为键（通过目录名），所以修改 slug 对题目包是破坏性变更。
- `id` 可选。有 `id` 时，它在题目包中必须唯一。它用于排序和查找。
- 目录必须使用规范名称：`id` 是十进制整数 n 时为 `<n 补足 4 位>-<slug>`（`id: "1"`、slug 为 `two-sum` 时是 `0001-two-sum`；`id: "12345"` 时是 `12345-<slug>`）；否则为 `<slug>`。

## problem.yaml

`problem.yaml` 是一个 YAML 映射。不认识的字段在任何层级都是错误。

| 字段 | 类型 | 必需 | 默认值 | 约束 | 起始版本 |
|---|---|---|---|---|---|
| `id` | 字符串 | 否 | 无 | 在题目包中唯一；见[标识](#标识)。YAML 整数按其十进制文本读取 | 1 |
| `slug` | 字符串 | 是 | | 字母、数字和单个短横线：`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`。应当全部小写。在题目包中唯一 | 1 |
| `title` | 字符串 | 是 | | 非空。使用题目包的默认语言区域 | 1 |
| `titles` | 映射 | 否 | `{}` | 题目包其他语言区域的标题，以语言区域为键：每个一项，不得为空；见[语言区域](#语言区域) | 1 |
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
| `source.site` | 字符串 | 否 | 无 | 任意文本，如 `leetcode.com` | 1 |
| `source.url` | 字符串 | 否 | 无 | | 1 |

与 `kind` 不对应的段（函数题中的 `class`、设计题中的 `function`）不得出现。

> **YAML 的坑。** 在 YAML 的流式写法（`{...}`、`[...]`）中，`[]` 本身是 YAML 语法，所以其中的数组类型必须加引号：`{name: nums, type: "integer[]"}`。用块状写法（`type: integer[]` 单独一行）时，引号可加可不加。

题目 0001：

```yaml
id: "1"
slug: two-sum
title: Two Sum
titles: {zh-CN: 两数之和}
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

- 在输入中，`null` 对每种 `Node` 类型都表示没有节点；输出把没有节点写作 `[]`。`MultilevelNode` 的输出以最后一个值结尾。
- Python 构造函数的参数与 LeetCode 一致，且都可省略：`Node(val, neighbors)`、`Node(val, children)`、`Node(x, next, random)`（存为 `val`）、`Node(val, left, right, next)`、`Node(val, isLeaf, topLeft, topRight, bottomLeft, bottomRight)`、`Node(val, prev, next, child)`。
- 解答可以返回自己创建的节点。返回的结构无法用这种编码表达时，算运行错误：`GraphNode` 不是一个值为 1 到 n（各出现一次）的图中的节点 1；随机指针指向返回链表之外的节点；`MultilevelNode` 的 `Prev` 没有指向前一个节点，或者某层第一个节点的 `Prev` 不为空；同一层有两个节点带子层。

### NestedInteger 和 Employee

- `NestedInteger` 存放一个整数，或一个由 `NestedInteger` 组成的列表。JSON：一个整数，或由 `NestedInteger` 值组成的数组：`[1,[4,[6]]]`。`NestedInteger[]` 是它们组成的数组（题目 0341：`[[1,1],2,[1,1]]`）。接口与 LeetCode 一致，所以解答也可以自己构造一个（LeetCode 385）：
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

题目 0146：

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

题目 0142（环形链表 II）给出 `head` 和 `pos`，解答只收到 `head`：

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

题目 0160（相交链表）：`headA` 和 `headB` 是包括共享尾部在内的完整链表：

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

题目 0236（二叉树的最近公共祖先）：`p` 和 `q` 在用例中是值，在解答中是 `root` 的节点：

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

题目 0237（删除链表中的节点）引用一个隐藏输入，并评判它：

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

题目 1379 的用例中没有 `cloned`：

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

题目 0133：

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

题目 0297（`separate`；0535 是 `same`）：

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
| `input` | 对象 | 没有 `generate` 时 | 用例的输入，按题目种类而定（见下文） | 1 |
| `expected` | JSON 值 | 否 | 期望输出，按题目种类而定（见下文）。`null` 也是一个值：`"expected": null` 是一个期望输出，缺少这个字段才表示没有 | 1 |
| `example` | 布尔值 | 否 | 该用例是题面中的示例 | 1 |
| `note` | 字符串 | 否 | 对用例的说明 | 1 |
| `generate` | 对象 | 代替 `input` | 该用例是生成的：由 `generator.py` 构造它的输入（[生成器](#生成器)） | 1 |
| `digest` | 字符串 | 与 `generate` 一起 | 生成的输入的摘要，即作者的生成器构造出的那个输入（[生成器](#生成器)） | 1 |

`generate` 只含以下字段：

| 字段 | 类型 | 必需 | 含义 | 起始版本 |
|---|---|---|---|---|
| `name` | 字符串 | 是 | `generator.py` 中构造输入的函数，是一个 Python 标识符 | 1 |
| `args` | 对象 | 否 | 它的关键字参数，以名字为键；默认 `{}`。`rng` 不能作参数名 | 1 |
| `seed` | 整数 | 否 | 它的 `rng` 的种子；默认 0 | 1 |

- 一行恰好包含一个 JSON 值。空行，以及去掉开头空白后以 `//` 开头的行，会被忽略。
- 用例从 1 开始编号，只计入包含用例的行。
- 没有 `expected` 的用例是合法的，但无法评判（[判题结果](#判题结果)）。
- 用例要么是存储的（有 `input`），要么是生成的（有 `generate`），不能两者都是。生成的用例没有 `expected`，因为它由参考解计算，也不能是 `example`。它的 `digest` 是可选的；没有它，就没有任何东西检查构造出的输入是否就是作者构造出的那个。
- 存储的用例（它在 `cases.jsonl` 中的那一行）应该不超过 100 KB；更大的输入应该改为生成。

按题目种类的 `input` 和 `expected`：

| 种类 | `input` | `expected` |
|---|---|---|
| 函数题 | 每个参数和隐藏输入各一个值，以名字为键，没有其他键。`ref` 参数取被引用节点的值，`copy` 参数不出现（[结构输入](#结构输入)） | 被评判的值：返回值、`judge.output` 的值（[原地修改的输出](#原地修改的输出)），或节点引用（`returns_ref`） |
| 设计题 | `{"ops": [...], "args": [...]}`，没有其他键。`ops` 是非空的名字数组：`ops[0]` 是类名（构造函数），其余是方法名。`args` 中每个操作对应一个数组，恰好包含该操作的参数 | 每个操作一项的数组：构造函数和 `void` 方法为 `null`，其余为返回值 |
| 编解码往返题 | 一个键，即 `encode` 的参数名，值为要编码的值 | `decode` 的结果 |

`judge.compare: any_of` 时，`expected` 改为由上述形式的所有可接受值组成的数组。`checker` 时，它是一个合法答案。各个值必须使用其声明类型的编码（[类型](#类型)）；`NextTreeNode` 的输出使用按层的形式。

题目 0146：

```json
{"input": {"ops": ["LRUCache","put","put","get","put","get","put","get","get","get"], "args": [[2],[1,1],[2,2],[1],[3,3],[2],[4,4],[1],[3],[4]]}, "expected": [null,null,null,1,null,-1,null,-1,3,4], "example": true}
```

题目 0001 的一个生成用例：

```json
{"generate": {"name": "distinct", "args": {"n": 10000}, "seed": 7}, "digest": "sha256:770f520998afe517410b3abd7ee67824586ddb091eb9035006a536f1d45da621", "note": "10^4 distinct numbers"}
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

- `judge.output: <输入>` 指向一个参数或隐藏输入。被评判的值是调用结束后它的值，按它的类型编码，而不是返回值。题目 0189（`rotate`，返回 `void`）评判 `nums`。
- `judge.output_length: return` 只评判 `judge.output` 的前 k 个元素，k 是返回值（LeetCode 的"前 k 个元素之后的内容不重要"）。它要求有 `judge.output`、其类型为数组，并且返回类型为 `integer` 或 `long`。返回的 k 不在 0 到数组长度之间时算运行错误。

题目 0027：

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

### 时间限制

解答的速度以同一种语言的参考解为准评判，两者在同一台机器上计时：

- 用例的用时是花在解答代码中的时间：函数题是函数调用本身，类题是构造函数和各次方法调用之和，不含参数的解码和结果的编码。
- 一次运行的**时间限制**是参考解在本次运行所选用例上的总用时的固定倍数，且不低于一个固定的下限。倍数和下限都属于评测端，而不属于题目，和[运行时限](#运行时限)一样；hc 的取值见它的文档。题目不声明时间限制：较慢的解答会不会超出，取决于题目的用例。
- 用例按顺序运行。运行的总用时在某个用例运行期间超过时间限制，或者某个用例的用时使总用时超过时间限制，这个用例判为超时（Time Limit Exceeded），运行随即停止：其后的用例为 Not Run。永不返回的用例在时间限制用完时被终止。
- 只有按其语言评判的解答（见[就绪](#就绪)）才有时间限制。参考解没有时间限制。

### 运行时限

解答在为它选定的用例上的一次运行有一个整次运行的墙钟时限，不是每个用例的时限。它用来防止卡死，比如解答卡在计时的调用之外；它不评判复杂度，那是[时间限制](#时间限制)的事。这个时限属于评测端，而不属于题目：hc 的时限是工作区 `hc.yaml` 中的 `timeout`，未配置时为 10s。到时后运行被终止：第一个没有结果的用例判为超时（Time Limit Exceeded），其后的用例为 Not Run，已经完成的用例保留各自的结果。判题程序的运行、生成器的运行，以及参考解为生成用例计算期望输出的运行，各自另有一个同样长度的时限。

### 就绪

有 `judge.compare` 的题目是**就绪**的。`judge.compare` 没有默认值，即使是 `exact` 也必须写出。不就绪的题目是合法的，但它的判题规则尚未确定。

题目就绪并且有某种语言的参考解（`reference/solution.py`、`reference/solution.go`）时，它**对这种语言就绪**；这种语言的解答以这份参考解为准计时（见[时间限制](#时间限制)）。题目对某种语言不就绪时，这种语言的解答的用例不作评判（Not Judged），也不能提交。题目应当对其题目包 `languages` 中的每种语言都就绪。

### 判题结果

一次运行对每个用例给出以下结果之一：

| 结果 | 何时 |
|---|---|
| Compile Error | 解答无法构建：Go 编译错误、Python 语法错误。这是整次运行的结果，没有用例会运行 |
| Runtime Error | 用例出错（异常、panic、驱动程序无法解码的输入，或者编码无法表达的结果），或者程序在该用例运行期间退出了 |
| Time Limit Exceeded | 运行在该用例期间用完了[时间限制](#时间限制)，或者用例给出结果之前[运行时限](#运行时限)已到 |
| Not Run | 运行在该用例之前就停止了：之前有用例超时，或者程序在之前的用例之后退出了 |
| No Expected Output | 用例运行了，但没有 `expected` |
| Not Judged | 用例运行了，但题目对解答的语言不是[就绪](#就绪)的 |
| Wrong Answer | 按 `judge.compare` 输出与 `expected` 不符，或者判题程序拒绝了它 |
| Accepted | 输出相符，或者判题程序接受了它 |

前四种结果在评判之前确定；No Expected Output 先于 Not Judged 确定。整次运行的结果是按用例顺序第一个结果为 Runtime Error、Time Limit Exceeded 或 Wrong Answer 的用例的结果，Not Run 的用例都在它之后。没有这样的用例时，只要有用例被接受（或者没有运行任何用例）就是 Accepted，否则有用例未评判时为 Not Judged，否则为 No Expected Output。

判题程序出错不是一种判题结果：见[判题程序](#判题程序)。无法构造的生成用例也不是：见[生成器](#生成器)。

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
- 对每个产生了输出并且有 `expected` 的用例，按用例运行的顺序调用一次 `check`，所有调用在解答运行结束后的同一个进程中进行，该进程的时限与[运行时限](#运行时限)长度相同。已经失败的用例（Runtime Error、Time Limit Exceeded）、没有运行的用例（Not Run）和没有 `expected` 的用例不会传入。
- 无法加载、没有可调用的 `check`、抛出异常、返回其他值或超时的判题程序，说明题目本身有问题，而不是对解答的判定：这次运行以错误结束，错误中注明 `checker.py` 和用例。
- 判题程序可以从 `hc_runtime` 导入公开名称（[Python](#python)）来重建结构，如 `decode("TreeNode", output)`；除此之外只能使用标准库。
- 判题程序必须接受每个正确的输出。对于随机题（LeetCode 384、528），输出要检查分布是否合理，这意味着正确的解答被拒绝的概率必须可以忽略不计。

题目 0005（最长回文子串，`"bab"` 和 `"aba"` 都正确）：

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

## 生成器

用例可以是生成的，而不是存储的：评判该用例时，由 `generator.py` 构造它的输入，由参考解计算它的期望输出。能抓出复杂度不对的解答的大输入就这样写，于是题目包里存的是构造数据的程序，而不是数据本身。

有生成用例的题目必须在目录中有 `generator.py`，没有生成用例的题目不得有。无论解答用什么语言，生成器都用 Python 编写，并且必须能在 Python 3.10 上运行。

```python
def descending(rng, n):
    return {"nums": list(range(n, 0, -1))}


def distinct(rng, n):
    return {"nums": rng.sample(range(-10**9, 10**9 + 1), n)}
```

- `generate.name` 指向 `generator.py` 中的一个函数，以 `name(rng, **args)` 的形式调用，`args` 即用例的 `generate.args`。它返回用例的输入：一个 `input` 形式的 dict（[用例](#用例)），值是解码后的 JSON 值，并且必须像存储的输入一样是该题目的合法输入。
- 生成器可以是任何程序，不只是随机的。精心设计的最坏情况（倒序数组、全部相等的值、退化的树）就是一个不用 `rng` 的函数。
- `rng` 是以 `generate.seed` 为种子的 `hc_runtime.Rng`：splitmix64，同一个种子在任何 Python 版本和机器上都给出相同的数。它的方法有 `random()`（[0, 1) 中的浮点数）、`randint(a, b)`（[a, b] 中的整数）、`uniform(a, b)`、`choice(seq)`、`shuffle(x)`（就地打乱一个列表）和 `sample(population, k)`（序列中 k 个互不相同的元素，顺序随机；`sample(range(1, 10**9), 10**5)` 不会构造整个 range）。生成器可以用 `Rng(seed)` 再创建别的。
- 生成器必须是确定的：同一个调用在任何机器和受支持的 Python 版本上都必须返回相同的输入。它们不得使用 `random` 模块（它只保证 `random()` 在不同 Python 版本间不变），也不得使用时间、环境、文件或网络，并且不得让以字符串为键的集合或字典的迭代顺序（哈希随机化会改变它）进入输入。
- `digest` 是 `sha256:` 加上输入的规范 JSON 的 SHA-256 的小写十六进制：UTF-8 的输入对象，键排序，没有空白，数字保持生成器写出的样子，字符串只转义 `"`、`\`、控制字符、U+2028 和 U+2029。它由工具写入（`hc cases build`），而不是由作者手写。与摘要不符的输入是作者从未检查过的用例，所以算题目出错。
- 生成用例的期望输出是参考解在其输入上的输出。有生成用例的题目必须有参考解。判题程序收到的 `expected` 就是这个输出。`judge.compare: any_of` 不能评判生成用例，因为参考解只找出一个答案：这类题目的大用例用 `unordered` 或判题程序评判。
- 一次运行中各生成用例的生成器在同一个进程里运行，在解答之前，时限为[运行时限](#运行时限)的长度。`generator.py` 无法加载、缺少该函数、抛出异常、返回的不是合法输入或与摘要不符，或者超时，都算题目出错，而不是对解答的判题结果；参考解在生成的输入上失败也一样：这次运行失败，并给出指明文件和用例的错误。
- 生成器可以导入 `hc_runtime` 的公开名称（[Python](#python)），此外只能使用标准库。

## 解答 ABI

解答必须是什么样子，驱动程序才能调用它。用户的解答、参考解（`reference/solution.*`）和初始代码（`templates/solution.*`）都是解答。参考解和初始代码必须遵守本节，并且只使用标准库；初始代码必须保持签名，即这里规定的函数名、类名、方法名、参数顺序和类型。解答按 `function.params`（或构造函数和方法参数）的顺序收到参数，不含隐藏输入，绑定方式见[类型](#类型)。

### Python

解答文件是 `solution.py`。

| | 函数题 | 设计题 |
|---|---|---|
| 入口 | 一个 `Solution` 类，带有名为 `function.name` 的方法，每个用例无参数实例化一次；或者在模块没有定义 `Solution` 时，一个名为 `function.name` 的模块级函数 | 一个名为 `class.name` 的类，`__init__` 接收构造函数参数，方法名为 `class.methods` 中的名字 |

- 未捕获的异常算运行错误；语法错误算编译错误。
- 协议的类型来自 `hc_runtime` 模块。它的公开 API 正是其 `__all__` 中的名称：`ListNode`、`TreeNode`、`GraphNode`、`NaryNode`、`RandomListNode`、`NextTreeNode`、`QuadNode`、`MultilevelNode`、`NestedInteger`、`Employee`、`Rng`、`decode` 和 `encode`。解答、判题程序和生成器可以导入这些名称，不得从中导入其他任何东西。`Rng` 是[生成器](#生成器)的随机数生成器。
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

参考解、初始代码、判题程序和生成器必须能在这些版本上运行，包括语言和标准库：只有在受支持的工具链上，才会用参考解计算期望输出或检查参考解。用户的解答可以使用其工具链支持的任何特性。要按 Go 1.22 检查 Go 代码，在它的模块中写 `go 1.22` 并运行 `go vet`，其中的 `stdversion` 检查会报告较新的标准库符号。

## 合法性规则

合法的题目包满足以下每一条规则，其中的每道题目也满足。`hc validate` 检查关于 `problem.yaml` 和 `cases.jsonl` 的规则，以及题目包中关于题目目录、标识和语言区域的规则；`hc validate --run` 还检查关于参考解和生成用例的规则。

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

**cases.jsonl**

1. 每个非空、非注释的行都必须是一个 JSON 对象，包含 `input`（对象），可选 `expected`、`example`（布尔值）和 `note`（字符串），没有其他字段；生成用例则包含 `generate`，可选 `digest` 和 `note`，没有其他字段。
2. 函数题用例的 `input` 必须恰好以参数（`copy` 参数除外）和隐藏输入为键；设计题用例恰好有 `ops` 和 `args`；编解码往返题用例恰好有 `encode` 的参数。
3. 每个值都必须使用其类型的编码（[类型](#类型)），包括树和评测系统定义的类型的约束：`TreeNode` 的层序只为实际存在的节点列出子节点、邻居在图中、随机下标在链表内、`NaryNode`、`QuadNode` 和 `MultilevelNode` 的形式合法、员工 id 互不相同且下属都已知。`ref` 的值必须是整数。
4. 结构输入必须吻合：`cycle` 的位置在链表内，被接上的两个链表从 `skip` 和 `at` 开始的值相同，`ref` 的值恰好出现一次。
5. 在设计题用例中，`ops` 和 `args` 的长度必须相同且不为零，`ops[0]` 必须是类名，其余每个操作都必须是方法，`args` 的每一项都必须恰好包含该操作的参数，且类型相符。
6. 有 `expected` 时，它必须具有被评判的值的形式（[用例](#用例)）：`any_of` 时为这种值组成的数组；设计题用例中每个操作一项，构造函数和 `void` 方法为 `null`。其中的值必须按输出的方式编码，规则 3 对输入没有这项要求：空的 `ListNode`、`TreeNode` 或 `Node` 不写作 `null`，没有末尾的 `null`，不是叶子的 `QuadNode` 写作 `[0,1]`。
7. `generate` 必须是一个对象，包含 `name`（Python 标识符），可选 `args`（键为 `rng` 以外的 Python 标识符的对象）和 `seed`（整数），没有其他字段。有 `digest` 时，它必须是 `sha256:` 加上 64 个小写十六进制数字。
8. 有生成用例时必须有 `generator.py`，否则不得有。用 `any_of` 评判的题目不得有生成用例。

**pack.yaml**

1. 必须是合法的 YAML，不得包含不认识的字段。
2. `protocol` 必须是正整数，题目包不得使用晚于它引入的特性（[协议版本](#协议版本)）。
3. `name` 必须匹配 `^[a-z0-9]+(-[a-z0-9]+)*$`，并且不得是 `local`。
4. 有 `version` 时，它必须是不带 `v` 前缀的 SemVer 2.0.0 版本号。
5. 有 `languages` 时，它不得为空，并且必须由所声明协议的语言 id 组成，每个只出现一次。
6. 每个分组都必须有匹配 `^[a-z0-9]+(-[a-z0-9]+)*$` 的 `name`，在分组中唯一，并且不得重复列出同一个 slug。
7. 有 `locales` 时，它不得为空，并且必须由 BCP 47 语言标签组成，每个只出现一次。`titles`（题目包的和每个分组的）的键必须是题目包中默认语言区域以外的语言区域，标题不得为空。

**题目包**

1. `pack.yaml` 必须位于题目包的根目录，题目必须位于 `problems/` 中（[题目包](#题目包)）。
2. 每个题目目录都必须使用规范名称（[标识](#标识)）。
3. `slug` 在题目包中必须唯一，有 `id` 时 `id` 也必须唯一。
4. 分组中的每一项都必须是题目包中某道题目的 slug。
5. 在有 `locales` 的题目包中，每道题目都必须有每个语言区域的题面和标题：默认语言区域是 `description.md` 和 `title`，其他语言区域是 `description.<locale>.md` 和非空的 `titles.<locale>`（[语言区域](#语言区域)）。
6. 题目的 `description.<locale>.md` 文件和 `titles` 的键必须是题目包中默认语言区域以外的语言区域；在没有 `locales` 的题目包中，题目两者都不得有。

**参考解、初始代码、判题程序和生成器**

1. 每份参考解都必须能构建，并且在受支持的工具链上，按题目的判题规则通过每个有 `expected` 的用例（Accepted；题目未就绪时为 Not Judged）；没有 `expected` 的用例必须能无错误地运行。有判题程序时，判题程序在任何参考解输出上都不得出错。
2. 有生成用例的题目必须有参考解。每个生成用例都必须能构造：它的函数必须返回合法的输入，有 `digest` 时就是摘要所指的那个；每份参考解都必须能在它上面无错误地运行，并且以任何其他参考解的输出为期望输出时能通过。
3. 参考解、初始代码、判题程序和生成器必须遵守[解答 ABI](#解答-abi)、[判题程序](#判题程序)和[生成器](#生成器)的规定，并且能在[支持的最低版本](#语言版本)上运行。`hc validate` 不检查这一条。
