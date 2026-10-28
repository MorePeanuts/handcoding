# handcoding 使用指南

[English](how-to-use.md) · 简体中文

- [工作区](#工作区)
- [交互式题目列表](#交互式题目列表)
- [命令](#命令)
- [获取题目](#获取题目)
- [用编码智能体生成用例](#用编码智能体生成用例)
- [调试](#调试)
- [配置](#配置)
- [目前的限制](#目前的限制)

## 工作区

`hc init` 创建工作区，默认位置是 `~/handcoding`：

```
~/handcoding/
  hc.yaml                     配置文件
  problems/0001-two-sum/      题库（见题目协议）
  solutions/go/0001-two-sum/  你的代码：solution.go，以及自动生成的 go.mod 和 zz_hc_driver.go
  solutions/python/...        你的代码：solution.py，以及自动生成的 hc_runtime.py 和 _hc_runner.py
  .hc/hc.db                   提交历史（SQLite）
  .hc/venv/                   hc 管理的 Python 虚拟环境
  pyrightconfig.json          让 pyright 使用 .hc/venv
```

`hc init --lang python` 把默认解题语言设为 Python（即 `hc.yaml` 中的 `language`，默认为 `go`），并立即创建下文介绍的 Python 环境。如果创建失败，例如既没有 uv 也没有 Python 3.10+，`hc init` 会给出警告，但仍然创建工作区。

`hc` 按以下顺序查找工作区：`-w <dir>`、`$HC_WORKSPACE`、包含 `hc.yaml` 的最近一级父目录、`~/handcoding`。所以在任何目录下都可以运行它。

每道 Go 题都是一个独立的 module，驱动文件就放在 `solution.go` 旁边。因此 gopls、`go build` 和 Delve 断点都直接作用在你自己的文件上。自动生成的文件每次运行前都会重写，不要手动修改。`go.mod` 跟随本机的 Go：其中的 `go` 行就是本机工具链的版本（Go 1.27.2 对应 `go 1.27`），所以你的解答可以使用它支持的所有特性。hc 用 `GOTOOLCHAIN=local` 构建，不会下载其他版本的工具链。

Python 解答在 hc 管理的虚拟环境 `.hc/venv` 中运行。第一次有命令需要 Python 时（`hc init --lang python`、`hc start`、`test`、`submit`、`debug`、`cases build`、`validate --run`），hc 会创建它：

1. 如果装了 [uv](https://docs.astral.sh/uv/)，用 `uv venv --python '>=3.10'` 创建；本机没有合适的 Python 时，uv 会自动下载一个；
2. 否则从 PATH 上的 `python3`、`python` 和 `python3.N` 中选出最新的 Python 3.10+，用 `-m venv` 创建。在 Debian 和 Ubuntu 上这需要安装 `python3-venv` 包。

环境中装有 debugpy，用于 headless 调试。下载 Python 和安装 debugpy 需要联网，但只需一次；判题始终离线。如果环境失效（工作区被移动，或者它所用的 Python 被删除），hc 会重新创建。`hc env sync` 可以随时重建它，例如在装好 uv 之后。hc 还会在工作区根目录写入 `pyrightconfig.json`（已有时不覆盖），让 pyright 和 basedpyright 用同一个解释器检查你的解答。如果想用自己的 Python（conda、自行编译的版本），在 `hc.yaml` 中设置 `python.interpreter`，hc 就不再创建虚拟环境。

建议在工作区里执行 `git init`，记录下自己的练习过程。

## 交互式题目列表

运行 `hc`（或 `hc list`）打开题目列表。最好把终端分成左右两栏：一边是 `hc`，一边是编辑器。

- 在题目上按 `enter` 打开题目：显示题面，根据模板创建解答文件，并把它的绝对路径**复制到剪贴板**。在编辑器里 `:e <粘贴>` 就可以开始写。
- 在题面页面可以测试、提交、调试和查看历史。结果显示在界面中，程序自己的输出单独列在下方。
- 调试器会接管终端：启动前清屏，退出后回到界面。如果上次运行有失败的用例，按 `d` 调试的就是那个用例。
- 题面图片直接画在终端里。kitty 和 Ghostty 显示原图；其他真彩色终端用块字符近似显示，会损失细节，可以按 `o` 用看图软件打开原图。显示方式见 [`images`](#配置)。
- 题面配色跟随终端背景色。如果颜色不对，在 `hc.yaml` 中设置 `theme`。

| 按键 | 操作 | 按键 | 操作 |
|---|---|---|---|
| `enter` | 打开题目（并复制解答路径） | `e` | 在编辑器中打开解答 |
| `t` | 用示例用例测试 | `s` | 提交（全部用例） |
| `d` | 调试（如有，调试上次失败的用例） | `h` | 提交历史：代码、判题详情、恢复、删除 |
| `y` | 再次复制解答路径 | `L` | 切换语言 |
| `r` | 重置为模板 | `D` | 放弃：删除解答及其历史，回到未开始状态 |
| `/` | 搜索 | `f` | 筛选：全部 / 未通过 / 已通过 |
| `o` | 打开题面中的图片（题面页面） | | |
| `esc` / `q` | 返回；在列表页按 `q` 退出 | `ctrl+c` | 退出 |

## 命令

```
练习          list  start  edit  show  test  submit  debug  reset  discard  history
题库          init  fetch  new  import  cases  validate
编码智能体    gen  fix  skill
```

题目可以用 slug、slug 前缀、编号或目录名指定：`hc test 1`、`hc test two`。

```sh
hc start two-sum -e                                 # 创建解答并打开编辑器
hc test two-sum                                     # 运行示例用例
hc test two-sum -c 4 -c 5                           # 运行指定用例
hc test two-sum --all -v                            # 全部用例，完整输出，不记录
hc test two-sum -i '{"nums":[1,2,3],"target":5}'    # 自定义输入（有参考解时会算出期望输出）
hc test lru-cache -l py                             # 用 Python 解题
hc submit two-sum                                   # 全部用例，记入历史

hc history two-sum          # 某道题的提交记录
hc history show 12          # 某次提交的代码和判题报告
hc history restore 12       # 把某次提交恢复为当前代码
hc history rm 12 13         # 删除提交记录
hc reset two-sum            # 重置为初始模板
hc discard two-sum          # 删除解答及其全部提交记录
```

运行 `hc <command> --help` 查看所有参数。

## 获取题目

### 从 LeetCode 获取

```sh
hc fetch 3 146 lru-cache    # 按编号、slug 或 URL，从 leetcode.com 获取
hc fetch --site cn 1        # 从 leetcode.cn 获取（默认中文题面）
hc fetch https://leetcode.cn/problems/two-sum/description/   # 任意题目 URL，站点由 URL 决定
hc fetch --site cn "LCR 001"                                  # leetcode.cn 的 LCR/LCP 等编号
hc gen 146                  # 让题目就绪：生成测试用例并决定判题规则
```

`hc fetch` 会下载题面及其图片、函数签名、Go 和 Python 的初始代码，以及示例用例。能从题面中解析出期望输出时，也会一并写入。

刚拉取的题目**还不能提交**。LeetCode 只公开示例，也不公开判题方式：顺序是否重要、是否有多个正确答案。在判题规则确定之前，`hc test` 会把你的输出和示例答案并排列出，但不给结论；`hc submit` 会拒绝提交。`hc gen` 会生成完整的测试用例，并根据题面决定判题规则，内置规则都不适用时编写判题程序；你也可以自己设置 `judge.compare`（见[题目协议](protocol_zh.md#problemyaml)）。题目列表会把这类题目标记为"not ready"。

会员题目无法获取。签名中使用了 hc 暂不支持的类型（例如交互题中的 `MountainArray`）的题目也无法获取，`hc fetch` 会说明原因。

### 自己出题

```sh
hc new my-problem --kind function   # 生成题目骨架（设计题用 --kind class）
$EDITOR ~/handcoding/problems/my-problem/problem.yaml
hc validate my-problem              # 边写边检查签名和用例
hc gen my-problem                   # 让智能体写参考解和用例，或者自己写：
hc cases build my-problem           #   用 reference/solution.* 生成期望输出
hc validate my-problem --run        #   每份参考解都必须通过每个用例
hc import ./my-problems/            # 导入别人分享的题目目录
```

[题目协议](protocol_zh.md#手写一道题)完整演示了一道函数题和一道设计题，并说明了参考解的写法。`hc import` 只复制协议中定义的文件；如果题库中已有相同的 slug 或编号，会拒绝导入（`--force` 会替换 slug 相同的题目）。

### 测试用例

```sh
hc cases list 3                     # 查看用例
hc cases add 3 -i '{"s":"abba"}'    # 追加一个用例（有参考解时会算出期望输出）
hc cases edit 3                     # 在编辑器中编辑 cases.jsonl
hc cases build 3                    # 用参考解补全缺失的期望输出（--force：全部重算）
```

## 用编码智能体生成用例

```sh
hc gen 3                                                   # 默认使用 Claude Code
hc gen 3 --agent codex -n 40 -m "more strings of one repeated character"
hc fix two-sum -c 5 -m "answers in a different order should pass"
```

`hc` 在工作区中以无头模式运行智能体（`claude -p` 或 `codex exec`）。启动前，它会安装或更新智能体要遵循的 skill **`hc-cases`**（见[智能体 skill](#智能体-skill)）。智能体按照 skill：

1. 编写参考解 `reference/solution.py`，必要时再写一个暴力解用来交叉验证；
2. 把测试**输入**写入 `cases.jsonl`，覆盖边界情况、特殊结构和大规模输入；
3. 运行 `hc cases build`，所以期望输出来自运行参考解，而不是模型自己写的；
4. 反复运行 `hc validate --run`，直到每份参考解都通过。

要使用其他智能体，在 `hc.yaml` 中设置它的命令行，`{prompt}` 会被替换为提示词：

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

### 智能体 skill

hc 自带若干 skill，向编码智能体讲解它的题目协议和工作流程。`hc init` 会把工作区该有的 skill（目前是 `hc-cases`：评判规则、参考解、checker 和测试用例）安装到 `.claude/skills/`（供 Claude Code）和 `.agents/skills/`（供 Codex），并在 `AGENTS.md` 的一个区块中列出它们。由于它们就在工作区里，在工作区中启动的任何 Claude Code 或 Codex 会话都能使用：让它"为这道 hc 题生成用例"即可。

```sh
hc skill list               # 每个 skill：是否安装、是否最新、是否过期或被修改，以及由哪个 hc 版本安装
hc skill install            # 安装或更新工作区的 skill（升级 hc 之后）
hc skill install --force    # 连你修改过的 skill 也一并替换
hc skill show hc-cases      # 打印一个 skill
```

hc 会给它安装的每一份副本做标记，记下版本和内容的哈希（skill 目录中的 `.hc-skill.json`），并且只替换或删除这些副本：你修改过的 skill 会保留，除非加上 `--force`；不是 hc 安装的 skill 从不触碰。在 `AGENTS.md` 中，hc 只改写 `<!-- hc:begin -->` 与 `<!-- hc:end -->` 之间的区块，文件的其余部分归你所有。`hc gen` 和 `hc fix` 在启动智能体前会安装缺失或过期的 skill；对于你修改过的 skill，只给出警告而不替换。不带名称运行 `hc skill install` 时，还会删除 hc 曾安装、但工作区已不再需要的 skill。

## 调试

```sh
hc debug two-sum -c 2       # Go：Delve，停在你的函数入口
hc debug lru -l py          # Python：pdb
```

### Neovim（nvim-dap）

```sh
hc debug two-sum -c 2 --headless    # Go：dlv headless，监听 127.0.0.1:38697
hc debug lru -l py --headless       # Python：debugpy，来自 .hc/venv
```

```lua
local dap = require("dap")
dap.adapters.hc = { type = "server", host = "127.0.0.1", port = 38697 }
table.insert(dap.configurations.go, { type = "hc", name = "hc attach", request = "attach", mode = "remote" })
table.insert(dap.configurations.python, { type = "hc", name = "hc attach", request = "attach" })
```

在解答文件中设好断点，启动 `hc debug --headless`，然后在 Neovim 中选择 "hc attach"。

## 配置

工作区根目录下的 `hc.yaml`：

```yaml
language: go          # 默认解题语言：go | python
editor: nvim          # 未设置时依次使用 $VISUAL、$EDITOR、vi
theme: auto           # 题面配色：auto | dark | light
images: auto          # 题面图片：auto | kitty | blocks | none
timeout: 10s          # 每次运行的墙钟时限；题目的 judge.timeout 会覆盖它
python:
  interpreter: python3.12  # 代替 .hc/venv 使用的 Python 3.10+，也可以是完整路径；默认不设置
agent:
  name: claude        # claude | codex，或者设置 command（见上文）
leetcode:
  site: com           # com | cn
```

`images: auto` 在 kitty、Ghostty 和 [herdr](https://herdr.dev) 中选择 `kitty`，在其他真彩色终端中选择 `blocks`，其余情况为 `none`。

- **herdr** 会自己绘制 kitty 图片，所以只要 herdr 外层的终端支持 kitty 图片协议，图片就能显示：kitty、Ghostty，或开启了 `enable_kitty_graphics = true` 的 WezTerm。在其他终端中图片区域是空白的，请设置 `images: blocks`，并按 `o` 查看原图。
- **tmux**：auto 模式从不选择 kitty。如果外层终端支持 Unicode 占位符（kitty、Ghostty），设置 `images: kitty` 并开启 tmux 的 `allow-passthrough`。
- **WezTerm** 支持 kitty 图片协议，但不支持 Unicode 占位符，所以 `images: kitty` 在其中会显示乱码。直接运行 WezTerm 时请保持 `blocks`。

## 目前的限制

- 交互题暂不支持。
- `hc fetch` 可以导入带[结构输入](protocol_zh.md#结构输入)的题目（带环链表、相交链表、二叉树的最近公共祖先这类节点引用），结构由 LeetCode 测试数据与初始代码之间的差异推出。不是普通 JSON 的示例输出（如 142 的 "tail connects to node index 1"）留给 `hc gen` 补写。示例题 141、142、160、236、237 和 1379 都属于这类题目。
- `hc fetch` 可以导入使用 [LeetCode 的 `Node` 类型](protocol_zh.md#leetcode-的-node-类型)、`NestedInteger`、`Employee` 的题目，签名取自 LeetCode 的 Go 初始代码。`returns_copy`（克隆图）留给 `hc gen` 设置；扁平化嵌套列表迭代器这类迭代器题目的示例只给出输入、没有给出调用序列，也留给 `hc gen` 补写。
- 把一个值编码再解码回来的题目（297、449、535）按[编解码往返题](protocol_zh.md#编解码往返题)评测：只比较解码得到的值，所以对编码本身的要求（449 的"尽可能紧凑"）不会检查。`hc fetch` 可以导入这类题目，编码器、解码器以及二者是否共用一个实例取自 LeetCode 初始代码注释中的调用方式。示例题 297 和 535 分别展示了两种情况。
- 会统计运行时间，但不限制内存。
- 无法获取 LeetCode 会员题目。
- 合法答案很多、无法全部列出的题目（"返回任意一个合法答案"）和随机题由[判题程序](protocol_zh.md#判题程序)评测，判题程序由 `hc gen` 编写，示例题 5、108、210、324、384 和 528 都带有判题程序。判题程序用 Python 编写，所以即使用 Go 作答，这些题目也需要 Python 3.10 或更新版本；hc 管理的虚拟环境会提供它。
