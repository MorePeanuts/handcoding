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
from typing import List


class Solution:
    def sumArray(self, nums: List[int]) -> int:
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
| `function` | 函数题的签名：`name`、`params`（`{name, type}` 列表）、`returns`（缺省为 `void`） |
| `class` | 设计题的签名：`name`、`constructor`（`{name, type}` 列表）、`methods`（`{name, params, returns}` 列表） |
| `judge.compare` | 输出的比较方式（见下文）。没有默认值：不写就表示题目*未就绪*（见下文） |
| `judge.tolerance` | 浮点误差。返回 `double` 时默认为 `1e-5`；绝对误差或相对误差满足其一即可 |
| `judge.output` | 原地修改类题目：调用结束后比较这个参数的值，而不是返回值 |
| `judge.output_length` | `return`：只判 `judge.output` 的前 k 个元素，k 是函数返回的整数。用于 LeetCode 的"删除重复项"这类题 |
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

`unordered` 和 `unordered_deep` 要求结果是数组。

### 类型

类型名沿用 LeetCode `metaData` 中的名字，所以从 LeetCode 获取的题目无需转换：

| 类型 | JSON | Go | Python |
|---|---|---|---|
| `integer` / `long` | 整数 | `int` / `int64` | `int` |
| `double` | 数字 | `float64` | `float` |
| `boolean` | `true` / `false` | `bool` | `bool` |
| `string` | 字符串 | `string` | `str` |
| `character` | 单字符字符串 `"a"` | `byte` | `str` |
| `ListNode` | 数组 `[1,2,3]` | `*ListNode` | `Optional[ListNode]` |
| `TreeNode` | 层序数组 `[3,9,20,null,null,15,7]` | `*TreeNode` | `Optional[TreeNode]` |
| `T[]` | 数组，最多 3 层；节点数组只支持 1 层 | `[]T` | `List[T]` |
| `void` | 只能作返回类型 | — | `None` |

`list<T>` 会规范化为 `T[]`；`int`、`bool`、`char`、`float` 会规范化为 `integer`、`boolean`、`character`、`double`。`Node`（图、N 叉树、带随机指针的链表）和 `NestedInteger` 等特殊类型暂不支持。

### cases.jsonl

每行一个用例：

```json
{"input": {"nums": [2,7,11,15], "target": 9}, "expected": [0,1], "example": true}
{"input": {"nums": [-1,-2,-3,-4,-5], "target": -8}, "expected": [2,4], "note": "negative numbers"}
```

- `input`：函数题是一个对象，每个参数恰好对应一个值，键为参数名。设计题是 `{"ops": [...], "args": [[...], ...]}`；`ops[0]` 必须是类名（即构造函数），`args` 的每一项恰好是对应操作的参数。
- `expected`：期望输出。设计题是每个操作返回值组成的数组，构造函数和 `void` 方法写 `null`。`any_of` 模式下是可接受答案组成的数组。缺省时由 `hc cases build` 用参考解计算。
- `example`：该用例是题面中的示例。`hc test` 默认只运行这些用例（一个都没标时运行前 3 个）。
- `note`：用例说明。
- 不允许出现其他字段。空行和以 `//` 开头的行会被忽略。用例从 1 开始编号，只计算真正包含用例的行。

### 参考解和初始代码

参考解的写法和用户的解答完全一样，放在 `reference/solution.py` 或 `reference/solution.go`。用 `hc start` 生成的初始代码就是去掉了函数体的参考解，可以照着写。

| | 函数题 | 设计题 |
|---|---|---|
| Python | `class Solution`，其中有名为 `function.name` 的方法；也可以是同名的模块级函数 | 名为 `class.name` 的类；`__init__` 接收构造函数参数；方法名与 `methods` 中一致 |
| Go | `package main`；`func <function.name>(...)`，参数类型见上面的类型表 | `type <Name> struct`、`func Constructor(...) <Name>`，以及指针接收者的方法，方法名首字母大写（`getMin` → `GetMin`） |

- `ListNode` 和 `TreeNode` 已经提供：Python 中用 `from hc_runtime import ListNode, TreeNode` 导入；Go 中已预先定义（字段为 `Val`、`Next`、`Left`、`Right`），不要自己再声明。
- 两份参考解都存在时，`hc cases build` 先用 Go，再用 Python；可以用 `-l python` 指定。`hc validate --run` 会运行每一份参考解。
- `templates/solution.go` 和 `templates/solution.py` 会替换自动生成的初始代码。它们必须保持签名不变，因为驱动要按签名调用。

### 校验

```
hc validate <slug>          # 按协议检查
hc validate <slug> --run    # 同时在每个用例上运行每一份参考解，必须全部通过
```

`hc validate` 会报告以下错误，并指出用例编号和字段：

- `problem.yaml`：语法错误（带行号和列号）、不认识的字段、不合法的 slug、缺少标题、不支持的类型、`judge.output` 不是参数、`void` 函数没写 `judge.output`、`judge.output_length` 用法不对、不认识的 `judge.compare`、不合法的 `judge.time_limit`；
- `cases.jsonl`：JSON 不合法、不认识的字段、缺少或多出的输入、值与声明的类型不符（`input "nums": [1]: must be integer, got string "x"`）、设计题中参数个数不对或方法不存在、`expected` 的形状不对；
- 加上 `--run` 时：参考解无法编译、运行出错，或与 `expected` 不一致（会同时显示两个值）。

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
    Check() error                     // 工具链是否可用
}
```

驱动只负责**解码输入、调用用户代码、编码输出**，从不判题。所有判题都在 Go 中完成（`internal/judge`），所以新语言不需要重新实现比较逻辑。

- `HC_CASES` 指向一个 JSON 文件，内容为 `[{"id": 1, "input": {...}}, ...]`。
- `HC_OUT` 指向一个文件，驱动以 JSON Lines 格式向其追加结果，每个用例一行：

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  `error` 非空表示运行时错误；尽量只保留用户代码中的调用栈。`elapsed_ns` 只统计用户代码的耗时。
- 用户的 stdout 和 stderr 直接输出到终端，所以可以用打印调试。
- 如果驱动崩溃或超时，没有结果行的用例分别判为 Runtime Error 或 Time Limit Exceeded。

现有的驱动可以作为参考：`internal/lang/golang`（生成代码加静态运行时）和 `internal/lang/python`（动态运行时，签名嵌入其中）。
