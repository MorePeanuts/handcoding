# handcoding (`hc`)

> [!WARNING]
> 本项目处于早期开发阶段（`v0.x`）。在 1.0 之前，命令行接口、配置文件、题目协议和数据格式都可能发生不兼容的变化，升级后可能需要重新初始化工作区。

本地刷题、本地判题的命令行工具。用你自己的编辑器（neovim、VS Code……）写代码，用 `hc` 在本地测试、提交、调试 —— 不需要联网，不需要付费调试器，测试用例完全可见。

- **本地题库 + 本地判题**：Go / Python，支持函数题、设计题（LRU Cache 这类）、原地修改题、链表、二叉树、字符矩阵、浮点误差、无序答案等常见题型
- **真正的调试器**：`hc debug` 直接启动 Delve / pdb 并停在你的函数入口；`--headless` 给 nvim-dap / VS Code 附加
- **LeetCode 导入**：`hc fetch 146` 拉取题面、签名、模板和示例（含期望输出）
- **测试用例由编码智能体生成**：`hc gen` 调用 Claude Code / Codex 写参考解与用例，再由 hc 用参考解计算期望输出并验证；`hc fix` 修复有问题的用例
- **提交历史**：每次 `hc submit` 记录代码与结果，可查看、恢复、删除
- **rustlings 风格的交互列表**：直接运行 `hc`

## 安装

```sh
go install github.com/MorePeanuts/handcoding/cmd/hc@latest          # 最新版本
go install github.com/MorePeanuts/handcoding/cmd/hc@v0.1.0-alpha.1  # 指定版本
# 或在仓库内
make install   # 安装到 $(go env GOBIN)，或 make build 生成 bin/hc
```

也可以从 [GitHub Releases](https://github.com/MorePeanuts/handcoding/releases) 下载预编译的二进制文件。

依赖：Go（写 Go 题）、Python 3.8+（写 Python 题）、可选 [Delve](https://github.com/go-delve/delve)（Go 调试）、debugpy（Python headless 调试）、`claude` 或 `codex`（生成用例）。

## 快速开始

```sh
hc init                    # 创建 ~/handcoding 工作区，附带 9 道示例题
hc                         # 打开交互式题目列表
hc start two-sum -e        # 生成模板并用编辑器打开（$EDITOR / hc.yaml 中的 editor）
hc test two-sum            # 运行示例用例
hc submit two-sum          # 运行全部用例并记录提交
hc debug two-sum -c 4      # 用第 4 个用例启动调试器
```

题目可以用 slug、slug 前缀、题号或目录名引用：`hc test 1`、`hc test two`。

### 交互界面

推荐用法：终端左右分屏，左边运行 `hc`，右边开编辑器。

- 在列表中按 `enter` 进入题目详情：显示题面，自动生成模板，并把解答文件的绝对路径**复制到剪贴板**，到右边 `:e <粘贴>` 即可开始写。
- 详情页里可以直接测试、提交、调试、查看历史。测试和提交的结果在界面内显示，程序自己的输出单独列在下方。
- 调试器需要独占终端，启动前会先清屏，退出后回到原界面。如果上一次运行有失败的用例，`d` 会直接调试那个用例。
- 题面的配色按终端背景自动选择。如果显示不对，在 `hc.yaml` 里设置 `theme: light` 或 `theme: dark`。
- 题面中的图片直接显示在终端里：kitty 和 Ghostty 显示原图，其他支持真彩色的终端用色块近似显示（细节会丢失），按 `o` 可以用系统看图软件打开原图。`hc.yaml` 中的 `images`（`auto` / `kitty` / `blocks` / `none`）可以指定显示方式，例如在 tmux 中使用 kitty 时需要设为 `kitty`，并开启 tmux 的 `allow-passthrough`。

| 键 | 作用 | 键 | 作用 |
|---|---|---|---|
| `enter` | 进入题目详情（并复制解答路径） | `e` | 用编辑器打开解答 |
| `t` | 测试示例用例 | `s` | 提交（全部用例） |
| `d` | 调试（优先选上次失败的用例） | `h` | 提交历史（可查看代码与判题详情、恢复、删除） |
| `y` | 再次复制解答路径 | `L` | 切换语言 |
| `r` | 重置为模板 | `D` | 放弃这道题：删除解答文件和提交历史，状态回到未做 |
| `/` | 搜索 | `f` | 过滤 全部/未完成/已解决 |
| `o` | 用系统看图软件打开题面图片（详情页） | | |
| `esc` / `q` | 返回上一层；在列表页按 `q` 退出 | `ctrl+c` | 退出 |

## 命令一览

```
练习      list  start  edit  show  test  submit  debug  reset  discard  history
题库      init  fetch  new  import  cases  validate
智能体    gen  fix  skill
```

常用示例：

```sh
hc test two-sum -c 4 -c 5                           # 指定用例
hc test two-sum --all -v                            # 全部用例，详细输出（不记录提交）
hc test two-sum -i '{"nums":[1,2,3],"target":5}'    # 自定义输入（有参考解时自动算期望输出）
hc test lru-cache -l py                             # 用 Python 做

hc history two-sum          # 提交历史
hc history show 12          # 查看某次提交的代码
hc history restore 12       # 把某次提交恢复为当前代码
hc history rm 12 13         # 删除提交记录
hc discard two-sum          # 删除解答文件和全部提交记录，状态回到未做

hc fetch 3 146 lru-cache    # 从 leetcode.com 拉题
hc fetch --site cn 1        # 从 leetcode.cn 拉题（默认使用中文题面）
hc cases list 3             # 查看用例
hc cases add 3 -i '{"s":"abba"}'    # 追加用例（有参考解时自动算期望输出）
hc cases edit 3             # 用编辑器改 cases.jsonl
```

## 测试用例生成：编码智能体

LeetCode 只公开示例用例，完整的测试集由编码智能体生成：

```sh
hc gen 3                    # 默认使用 Claude Code（headless 模式）
hc gen 3 --agent codex -n 40 -m "多覆盖全是相同字符的情况"
hc fix two-sum -c 5 -m "期望输出顺序不同也应判对"
```

工作方式：hc 在工作区中以无头模式运行智能体（`claude -p` / `codex exec`），并把 **handcoding skill** 安装到工作区（`.claude/skills/handcoding/SKILL.md`，以及给 Codex 的 `AGENTS.md`）。智能体按 skill 中的流程：

1. 写参考解 `reference/solution.py`（必要时再写暴力解交叉验证）；
2. 在 `cases.jsonl` 中写入覆盖边界、特殊结构、大规模数据的 **输入**；
3. 运行 `hc cases build` —— 期望输出由 hc 执行参考解得到，而不是让模型"心算"；
4. 运行 `hc validate --run` 直到通过。

关于 A2A：A2A 是面向网络服务的智能体互操作协议，对"本地 CLI 调用本地智能体"来说过重。这里采用 子进程 + skill 的方式，智能体通过调用 `hc` 子命令完成验证闭环，效果等价且零部署。也可以 `hc skill install --global` 后，在任意 Claude Code 会话里直接让它"给 hc 的某道题生成用例"。

自定义智能体：在 `hc.yaml` 中设置 `agent.command: ["my-agent", "--yes", "{prompt}"]`。

## 自定义题目

```sh
hc new my-problem --kind function     # 生成题目骨架
$EDITOR ~/handcoding/problems/my-problem/problem.yaml
hc validate my-problem
hc gen my-problem                      # 让智能体写参考解和用例
hc import ./my-problems/               # 导入他人分享的题目目录
```

格式详见 [docs/protocol.md](docs/protocol.md)。

## 工作区结构

```
~/handcoding/
  hc.yaml                     配置：默认语言、编辑器、超时、智能体、LeetCode 站点
  problems/0001-two-sum/      题库（题目协议）
  solutions/go/0001-two-sum/  你的代码：solution.go + 生成的 go.mod / zz_hc_driver.go
  solutions/python/...        你的代码：solution.py + 生成的 hc_runtime.py / _hc_runner.py
  .hc/hc.db                   提交历史（SQLite）
```

每道 Go 题是一个独立的 module，solution.go 与驱动在同一目录，所以 gopls 补全、`go build`、Delve 断点都直接作用在你的真实文件上。建议对工作区 `git init`，记录自己的练习过程。

### neovim 调试（nvim-dap）

```sh
hc debug two-sum -c 2 --headless          # Go：dlv headless，监听 127.0.0.1:38697
hc debug lru -l py --headless             # Python：debugpy（需 pip install debugpy）
```

```lua
local dap = require("dap")
dap.adapters.hc = { type = "server", host = "127.0.0.1", port = 38697 }
table.insert(dap.configurations.go, { type = "hc", name = "hc attach", request = "attach", mode = "remote" })
table.insert(dap.configurations.python, { type = "hc", name = "hc attach", request = "attach" })
```

在 solution 文件里打好断点，启动 `hc debug --headless`，然后在 neovim 中选择 "hc attach"。

## 开发

```sh
make test     # 单元测试 + 端到端测试（会用 Go 与 Python 参考解互相校验所有内置题）
make lint     # go vet + golangci-lint
make build
```

代码结构：

```
cmd/hc                  入口
internal/cli            cobra 命令
internal/tui            bubbletea 交互列表
internal/app            用例层：开始/测试/提交/参考解/用例构建
internal/problem        题目协议：problem.yaml、cases.jsonl、类型系统
internal/lang           语言接口；golang/、python/ 为具体实现
internal/runner         构建、运行驱动、收集结果
internal/judge          结果比较（exact / unordered / unordered_deep / any_of / 浮点容差）
internal/store          提交历史（SQLite，纯 Go 驱动，无 cgo）
internal/leetcode       LeetCode GraphQL 客户端与格式转换
internal/agent          编码智能体调用
internal/builtin        内置示例题与 skill（embed）
```

## 现阶段的限制

- 暂不支持 `Node`（图、N 叉树、随机指针链表）、`NestedInteger`、带环链表输入等特殊签名，以及需要特殊判题器（checker）的题目
- 只统计运行时间，不限制内存
- LeetCode 会员题无法拉取

## 许可证

[MIT](LICENSE)
