# handcoding 协议

本文档定义两套协议：

1. **题目协议**：题库中一道题在磁盘上的格式，用于自定义题目导入（`hc new` / `hc import`）。
2. **驱动协议**：hc 与各语言测试驱动之间的约定，用于扩展新语言。

---

## 1. 题目协议

### 目录结构

```
problems/0001-two-sum/          目录名：<4 位编号>-<slug>，无编号时为 <slug>
  problem.yaml                  元数据、函数签名、判题选项（必需）
  description.md                题面，Markdown
  cases.jsonl                   测试用例，每行一个 JSON 对象
  reference/solution.py         参考解（可选，可多语言），用于计算期望输出
  reference/solution.go
  templates/solution.go         起始模板（可选，缺省时按签名生成）
  templates/solution.py
```

### problem.yaml

| 字段 | 说明 |
|---|---|
| `id` | 题号（字符串，可选），用于排序与引用 |
| `slug` | 唯一标识，小写 kebab-case（必需） |
| `title` | 标题（必需） |
| `difficulty` | `easy` / `medium` / `hard` |
| `tags` | 标签列表 |
| `kind` | `function`（函数题）或 `class`（设计题） |
| `function` | 函数题签名：`name`、`params[{name,type}]`、`returns` |
| `class` | 设计题签名：`name`、`constructor[{name,type}]`、`methods[{name,params,returns}]` |
| `judge.compare` | 比较方式，见下表，默认 `exact` |
| `judge.tolerance` | 浮点容差；返回 `double` 时默认 `1e-5`（绝对或相对误差满足其一即可） |
| `judge.output` | 原地修改类题目：返回 `void`，判题比较调用后该参数的值 |
| `judge.time_limit` | 整次运行的时限，如 `5s`，默认取 hc.yaml 的 `timeout` |
| `source` | `site`、`url`，来源信息 |

比较方式：

| `compare` | 语义 | 例题 |
|---|---|---|
| `exact` | JSON 值完全相等（数字按数值比较） | 大多数题 |
| `unordered` | 最外层数组视为多重集合，忽略顺序 | 两数之和、全排列 |
| `unordered_deep` | 每一层数组都忽略顺序 | 字母异位词分组、子集 |
| `any_of` | `expected` 是所有可接受答案组成的数组 | 答案集合较小的多解题 |

### 类型系统

沿用 LeetCode `metaData` 的类型名，从 LeetCode 拉取的题目无需转换：

| 类型 | JSON 表示 | Go | Python |
|---|---|---|---|
| `integer` / `long` | 数字 | `int` / `int64` | `int` |
| `double` | 数字 | `float64` | `float` |
| `boolean` | `true/false` | `bool` | `bool` |
| `string` | 字符串 | `string` | `str` |
| `character` | 单字符字符串 `"a"` | `byte` | `str` |
| `ListNode` | 数组 `[1,2,3]` | `*ListNode` | `Optional[ListNode]` |
| `TreeNode` | 层序数组 `[3,9,20,null,null,15,7]` | `*TreeNode` | `Optional[TreeNode]` |
| `T[]` | 数组，最多 3 层；节点数组 1 层 | `[]T` | `List[T]` |
| `void` | 仅作返回值 | — | `None` |

`list<T>` 会被规范化为 `T[]`。暂不支持 `Node`（图/N 叉树/带 random 指针链表）、`NestedInteger` 等特殊类型。

### cases.jsonl

每行一个用例：

```json
{"input": {"nums": [2,7,11,15], "target": 9}, "expected": [0,1], "example": true}
{"input": {"nums": [-1,-2,-3,-4,-5], "target": -8}, "expected": [2,4], "note": "negative numbers"}
```

- `input`：函数题为 参数名 → 值；设计题为 `{"ops": [...], "args": [[...], ...]}`，`ops[0]` 必须是类名（构造函数）。
- `expected`：期望输出。设计题为每个操作的返回值数组，构造函数与 `void` 方法为 `null`。缺省时可由 `hc cases build` 用参考解计算。
- `example`：题面示例，`hc test` 默认只跑这些。
- `note`：用例说明。
- 空行和 `//` 开头的行会被忽略。用例编号为有效行的 1-based 序号。

### 校验

```
hc validate <slug>          # 协议检查
hc validate <slug> --run    # 另外用参考解跑全部用例，必须全部通过
```

---

## 2. 驱动协议（扩展新语言）

每种语言实现 `internal/lang.Language` 接口，并在 `internal/app.Languages` 中注册：

```go
type Language interface {
    ID() string                       // "go"，用于路径和 --lang
    Name() string
    Aliases() []string
    SolutionFile() string             // 用户编辑的文件名，如 solution.go
    Starter(p *problem.Problem) (string, error)            // 起始代码
    Prepare(dir string, p *problem.Problem) error          // 写入驱动/运行时文件（每次运行前调用，需幂等）
    Build(ctx context.Context, dir, binDir string) error   // 编译或语法检查，失败返回 *lang.CompileError
    Command(ctx context.Context, dir, binDir string) *exec.Cmd
    DebugCommand(dir, binDir string, p *problem.Problem, opts lang.DebugOptions) (*exec.Cmd, error)
    Check() error                     // 工具链是否可用
}
```

驱动程序只负责 **解码输入 → 调用用户代码 → 编码输出**，不做任何判题；判题统一在 Go 侧（`internal/judge`）完成，因此新语言无需重复实现比较逻辑。

- 环境变量 `HC_CASES`：一个 JSON 文件，内容为 `[{"id": 1, "input": {...}}, ...]`。
- 环境变量 `HC_OUT`：驱动按 JSON Lines 写入结果，每个用例一行：

  ```json
  {"id": 1, "output": [0,1], "error": "", "elapsed_ns": 1234}
  ```

  `error` 非空表示运行时错误（建议只保留用户代码中的调用栈）；`elapsed_ns` 只统计用户代码耗时。
- 用户代码的 stdout/stderr 直接透传到终端，方便 printf 调试。
- 驱动进程崩溃或超时时，未写出结果的用例分别判为 Runtime Error / Time Limit Exceeded。

现有实现可作为参考：`internal/lang/golang`（代码生成 + 静态运行时）与 `internal/lang/python`（动态运行时 + 内嵌签名）。
