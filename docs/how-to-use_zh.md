# handcoding 使用指南

[English](how-to-use.md) · 简体中文

- [工作区](#工作区)
- [交互式题目列表](#交互式题目列表)
- [命令](#命令)
- [获取题目](#获取题目)
- [题目包](#题目包)
- [用编码智能体生成用例](#用编码智能体生成用例)
- [调试](#调试)
- [配置](#配置)
- [目前的限制](#目前的限制)

## 工作区

`hc init` 创建一个空的工作区，默认位置是 `~/handcoding`：

```
~/handcoding/
  hc.yaml                                  配置文件，以及已安装题目包的列表
  packs/local/problems/0001-two-sum/       你自己的题目（见题目协议）
  packs/<name>/                            已安装的题目包（只读）
  solutions/local/go/0001-two-sum/         你的代码：solution.go，以及自动生成的 go.mod 和 zz_hc_driver.go
  solutions/local/python/0001-two-sum/     你的代码：solution.py，以及自动生成的 hc_runtime.py 和 _hc_runner.py
  .hc/hc.db                                提交历史（SQLite）
  .hc/venv/                                hc 管理的 Python 虚拟环境
  pyrightconfig.json                       让 pyright 使用 .hc/venv
```

`hc init --lang python` 把默认解题语言设为 Python（即 `hc.yaml` 中的 `language`，默认为 `go`），并立即创建下文介绍的 Python 环境。如果创建失败，例如既没有 uv 也没有 Python 3.10+，`hc init` 会给出警告，但仍然创建工作区。

`hc` 按以下顺序查找工作区：`-w <dir>`、`$HC_WORKSPACE`、包含 `hc.yaml` 或 `pack.yaml`（题目包的检出目录，见[编写题目包](#编写题目包)）的最近一级父目录、`~/handcoding`。所以在任何目录下都可以运行它。

### 工作区中的题目包

工作区里的题目按[题目包](protocol_zh.md#题目包)组织，每个题目包是 `packs/` 下的一个目录：

- `packs/local/` 存放你自己的题目，也就是 `hc fetch`、`hc new` 和 `hc import` 添加的题目。它没有 `pack.yaml`（`local` 是保留名称），可以用所有语言作答。
- 其余每个目录都是一个已安装的题目包：一个 `name` 与目录名相同的 `pack.yaml`，加上它的 `problems/`。已安装的题目包是**只读**的：`hc gen`、`hc fix` 和 `hc cases add|rm|edit|build` 拒绝修改它们，因为更新题目包会覆盖这些修改。`pack.yaml` 列出了 `languages` 的题目包只能用这些语言作答：`hc start -l` 拒绝其他语言；如果你的默认语言不在其中，hc 会改用题目包的第一种语言并给出提示。`hc pack install` 负责安装题目包（见[题目包](#题目包)）；手动复制到 `packs/` 下的题目包也能用，`hc pack list` 会把它显示为 unmanaged（未托管）。

hc 无法读取的题目包（没有 `pack.yaml`、`pack.yaml` 不合法，或为更新的协议编写，此时会提示升级 hc）会由 `hc validate` 和题目列表报告，它的题目不会被加载；工作区的其余部分照常工作。

解答存放在 `solutions/<pack>/<lang>/<dir>/`，提交历史记录每次提交的题目包和 slug，所以两个题目包里相同的 slug 是两道不同的题。

旧版 hc 创建的工作区把题目放在 `problems/` 而不是 `packs/` 下。hc 遇到这种工作区会停下并说明：用 `hc init <dir>` 新建一个工作区，再把你的题目移到它的 `packs/local/problems/` 下。解答和提交历史不会迁移。

每道 Go 题都是一个独立的 module，驱动文件就放在 `solution.go` 旁边。因此 gopls、`go build` 和 Delve 断点都直接作用在你自己的文件上。自动生成的文件每次运行前都会重写，不要手动修改。`go.mod` 跟随本机的 Go：其中的 `go` 行就是本机工具链的版本（Go 1.27.2 对应 `go 1.27`），所以你的解答可以使用它支持的所有特性。hc 用 `GOTOOLCHAIN=local` 构建，不会下载其他版本的工具链。

Python 解答在 hc 管理的虚拟环境 `.hc/venv` 中运行。第一次有命令需要 Python 时（`hc init --lang python`、`hc start`、`test`、`submit`、`debug`、`cases build`、`validate --run`），hc 会创建它：

1. 如果装了 [uv](https://docs.astral.sh/uv/)，用 `uv venv --python '>=3.10'` 创建；本机没有合适的 Python 时，uv 会自动下载一个；
2. 否则从 PATH 上的 `python3`、`python` 和 `python3.N` 中选出最新的 Python 3.10+，用 `-m venv` 创建。在 Debian 和 Ubuntu 上这需要安装 `python3-venv` 包。

环境中装有 debugpy，用于 headless 调试。下载 Python 和安装 debugpy 需要联网，但只需一次；判题始终离线。如果环境失效（工作区被移动，或者它所用的 Python 被删除），hc 会重新创建。`hc env sync` 可以随时重建它，例如在装好 uv 之后。hc 还会在工作区根目录写入 `pyrightconfig.json`（已有时不覆盖），让 pyright 和 basedpyright 用同一个解释器检查你的解答。如果想用自己的 Python（conda、自行编译的版本），在 `hc.yaml` 中设置 `python.interpreter`，hc 就不再创建虚拟环境。

支持的最低版本是 Python 3.10 和 Go 1.22（见协议的[语言版本](protocol_zh.md#语言版本)）。你自己的解答可以使用工具链支持的任何特性：`hc test`、`hc submit` 和 `hc debug` 会注明所用的工具链（`[Python 3.12.9]`），版本过旧时只给出警告。题目附带的代码必须能在最低版本上运行，所以 `hc cases build` 和 `hc validate --run` 拒绝在过旧的工具链上运行参考解。更新的工具链没有问题：`hc validate --run`（因而 `hc pack check`）还会对照 Go 1.22 检查每个 Go 参考解，做法是在 go 指令为 `go 1.22` 的模块中对它运行 go vet，它会报告更新的语言特性和标准库符号。

建议在工作区里执行 `git init`，记录下自己的练习过程。`hc init` 写入的 `.gitignore` 排除了 `.hc/` 和已安装的题目包，后者可以由 `hc pack install` 根据 `hc.yaml` 重新安装。

### 编写题目包

要创建自己的题目包，运行 `hc pack init`：它写入 `pack.yaml`、`.gitignore`、CI 工作流和智能体文件，可以直接提交。创建、检查和发布题目包的指南见[编写题目包](packs_zh.md)。

题目包的检出目录，即包含 `pack.yaml` 的目录（例如克隆下来的题目包仓库），本身就是一个工作区：在它或它下面的任意目录中运行 hc，或者用 `-w` 指向它即可。它不需要 `hc init`，`hc init` 也会拒绝在题目包内的目录中创建工作区。在这种编写用的工作区里，这个题目包是唯一的题目包，并且可写：`hc fetch`、`hc new` 和 `hc import` 把题目加到它的 `problems/` 下，`hc gen`、`hc fix` 和 `hc cases` 修改这些题目。所以编写流程不需要任何参数：`hc fetch two-sum`、`hc gen two-sum`、`hc validate --run`，发布前再运行 `hc pack check`（见[检查题目包](#检查题目包)）。这里没有 `local` 题目包，检出目录中的 `packs/` 目录会被忽略，题目只用 slug 来指代。

```
my-pack/
  pack.yaml  problems/                     题目包本身（见题目协议）
  hc.yaml                                  可选：你的个人设置（language、editor、agent、timeout、python）
  solutions/<pack>/<lang>/<dir>/           你的解答，与练习工作区相同
  .hc/  pyrightconfig.json                 hc 的状态，与练习工作区相同
  .claude/  .agents/  AGENTS.md            智能体技能，在某个命令安装之后出现
```

没有 `hc.yaml` 时 hc 使用默认设置，并且从不在检出目录中创建它。hc 无法读取的 `pack.yaml`（例如为更新的协议编写）不会妨碍工作区打开：`hc validate` 会指出问题所在。hc 添加的这些内容属于你而不属于题目包，所以不要把它们提交到仓库：

```
# .gitignore
.hc/
solutions/
hc.yaml
pyrightconfig.json
```

`hc pack init` 写入的就是这个 `.gitignore`，并且把 `.claude/`、`.agents/` 和 `AGENTS.md` 留在仓库中，让新克隆的仓库能向智能体说明自己。用其他方式创建的题目包，可以把它们也加入忽略列表，或者提交它们（hc 只修改它自己安装的技能和 `AGENTS.md` 中它自己的那一段）。

安装在练习工作区中的题目包（与 `hc.yaml` 同级的 `packs/<name>/`）不是检出目录：在其中运行 hc 时，使用的是外层的工作区，题目包在那里仍然只读。

## 交互式题目列表

运行 `hc`（或 `hc list`）打开题目列表。最好把终端分成左右两栏：一边是 `hc`，一边是编辑器。

- 在题目上按 `enter` 打开题目：显示题面，根据模板创建解答文件，并把它的绝对路径**复制到剪贴板**。在编辑器里 `:e <粘贴>` 就可以开始写。
- 在题面页面可以测试、提交、调试和查看历史。结果显示在界面中，程序自己的输出单独列在下方。
- 调试器会接管终端：启动前清屏，退出后回到界面。如果上次运行有失败的用例，按 `d` 调试的就是那个用例。
- 题面图片直接画在终端里。kitty 和 Ghostty 显示原图；其他真彩色终端用块字符近似显示，会损失细节，可以按 `o` 用看图软件打开原图。显示方式见 [`images`](#配置)。
- 题面配色跟随终端背景色。如果颜色不对，在 `hc.yaml` 中设置 `theme`。
- 题目来自多个题目包时，列表会显示每道题所属的题目包，搜索也会匹配 `pack/slug`（输入 `/local/` 可以找到你自己的题目）。`hc list --pack <name>` 和 `--group <name>` 打开的列表只包含某个题目包或某个题目包里的一个分组。

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
题目          init  fetch  new  import  cases  validate  pack  docs
编码智能体    gen  fix  skill
```

题目可以用 slug、slug 前缀、编号或目录名指定：`hc test 1`、`hc test two`。这样会在所有题目包中查找；`<pack>/<ref>` 只在一个题目包中查找：`hc test local/two-sum`、`hc test leetcode/1`。如果一个 slug（或编号）出现在多个题目包中，不带题目包的引用会报错并以 `pack/slug` 列出它们；前缀必须在所有题目包中只匹配一个 slug。

```sh
hc start two-sum -e                                 # 创建解答并打开编辑器
hc test two-sum                                     # 运行示例用例（没有标记示例时运行前 3 个用例）
hc test two-sum -c 4 -c 5                           # 运行指定用例
hc test two-sum --all -v                            # 全部用例，完整输出，不记录
hc test two-sum -i '{"nums":[1,2,3],"target":5}'    # 自定义输入（有参考解时会算出期望输出）
hc test lru-cache -l py                             # 用 Python 解题
hc submit two-sum                                   # 全部用例，记入历史

hc list --plain --pack leetcode --group arrays   # 以表格列出某个题目包的一个分组，按分组中的顺序

hc history                  # 全部提交记录（有多个题目包时，题目显示为 pack/slug）
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

刚拉取的题目**还不能提交**。LeetCode 只公开示例，也不公开判题方式：顺序是否重要、是否有多个正确答案。在判题规则确定之前，`hc test` 会把你的输出和示例答案并排列出，但不给结论；`hc submit` 会拒绝提交。`hc gen` 会生成完整的测试用例，并根据题面决定判题规则，内置规则都不适用时编写判题程序；你也可以自己设置 `judge.compare`（见协议的[比较方式](protocol_zh.md#比较方式)）。题目列表会把这类题目标记为"not ready"。

会员题目无法获取。签名中使用了 hc 暂不支持的类型（例如交互题中的 `MountainArray`）的题目也无法获取，`hc fetch` 会说明原因。

### 自己出题

```sh
hc new my-problem --kind function   # 生成题目骨架（设计题用 --kind class）
$EDITOR ~/handcoding/packs/local/problems/my-problem/problem.yaml
hc validate my-problem              # 边写边检查签名和用例
hc gen my-problem                   # 让智能体写参考解和用例，或者自己写：
hc cases build my-problem           #   用 reference/solution.* 生成期望输出
hc validate my-problem --run        #   每份参考解都必须通过每个用例
hc import ./my-problems/            # 导入别人分享的题目目录，或一个题目包里的题目
```

`hc fetch`、`hc new` 和 `hc import` 都写入 `packs/local/problems/`。题目的格式，从 `problem.yaml` 到参考解，由[题目协议](protocol_zh.md)规定；`hc new` 会生成一个待填写的骨架，hc 的回归测试集（仓库中的 [`testdata/regression`](../testdata/regression)）则包含覆盖每项功能的完整例子。

- `hc cases build` 运行一个参考解：有 Go 参考解时用 Go，否则用 Python；`-l` 可以指定。判题方式为 `any_of` 的题目会被拒绝，因为它的 `expected` 要列出所有可接受的答案，只能手写。
- `hc validate` 把每一处违反协议[合法性规则](protocol_zh.md#合法性规则)的地方报为错误，并注明用例编号和字段。加上 `--run` 时，它还会在每个用例上运行每个参考解，有判题程序时交给判题程序判定。以下情况给出警告：判题规则未确定（没有 `judge.compare`）、没有用例、有用例缺少 `expected`、没有用例标记为示例、没有参考解，以及题面引用的图片不存在。`hc test`、`hc submit` 和 `hc debug` 拒绝运行违反这些规则的用例，并指出是哪一个。
- `hc import` 只复制协议定义的条目（其余内容，如编辑器文件、`solutions/` 目录，不会被导入）；如果你自己的题目中已有相同的 slug 或编号，会拒绝导入（`--force` 会替换 slug 相同的题目）。slug 和编号只需在一个题目包内唯一，所以导入一道已安装题目包里也有的题目没有问题。
- 不带参数的 `hc validate` 还会检查每个已安装题目包的 `pack.yaml`，以及它的分组是否都指向本题目包的题目。每道题的目录名、slug 和编号都会与所在题目包的其他题目对照检查。

hc 内置了与自身版本对应的协议（英文版），离线也能查阅：

```sh
hc docs protocol                    # 完整协议
hc docs protocol --list             # 列出各节
hc docs protocol structure-inputs   # 某一节，包括其下的小节
```

每一节用英文版标题在 GitHub 上的锚点命名，如 `protocol.md#structure-inputs`。在终端中输出会像题面一样渲染；通过管道输出或加上 `--raw` 时输出 Markdown 原文。

### 测试用例

```sh
hc cases list 3                     # 查看用例
hc cases add 3 -i '{"s":"abba"}'    # 追加一个用例（有参考解时会算出期望输出）
hc cases edit 3                     # 在编辑器中编辑 cases.jsonl
hc cases build 3                    # 用参考解补全缺失的期望输出（--force：全部重算）
```

## 题目包

[题目包](protocol_zh.md#题目包)是一个带有 `pack.yaml` 的现成题目目录，通常是一个 git 仓库。hc 调用 PATH 上的 `git` 来安装和更新题目包。

```sh
hc pack install https://github.com/someone/handcoding-pack.git   # 把题目包克隆到 packs/<name>/
hc pack install ../my-pack --ref v1.2.0                          # 本地 git 仓库，指定 tag 或分支
hc pack list                                # 题目包、版本、commit、来源和你的进度
hc pack update                              # 更新所有题目包（也可以指定名字）
hc pack update leetcode --ref main          # 改为跟随另一个 tag 或分支
hc pack remove leetcode                     # 删除题目包；你的解答和提交历史保留
hc pack remove leetcode --purge             # ……连同它们一起删除
```

> [!WARNING]
> 安装题目包意味着信任它的代码：测试、提交或校验时，它的参考解和 checker 会在你的机器上运行。只从你信任的来源安装题目包。

- **`hc pack install <source>`** 接受 git URL 或目录。本身是 git 仓库的目录会像 URL 一样被克隆；其他包含 `pack.yaml` 的目录会被复制（只复制 `pack.yaml` 和 `problems/`）。`--ref` 安装 git 来源的某个 tag 或分支，而不是默认分支。题目包安装到 `packs/<name>/`，名字取自它的 `pack.yaml`。hc 拒绝不合法的 `pack.yaml`、为它不支持的协议版本编写的 `pack.yaml`（并说明该升级 hc 还是题目包），以及与 `packs/` 中已有题目包重名的题目包。加载失败的题目会被报告，但不会阻止安装。
- **`hc pack list`** 显示每个题目包的名字、版本、题目数、进度（有通过提交的题目数）、已安装的 commit 和来源。`packs/` 中 `hc.yaml` 没有列出的目录（例如手动复制进来的）标为 *unmanaged*（未托管）；`hc.yaml` 列出了但 `packs/` 中没有的题目包标为 *missing*（缺失）。
- **`hc pack update [name]...`** 拉取每个题目包，并检出它的 ref 的最新 commit：分支会前进，tag 保持不动（除非上游移动了它），`--ref` 改为跟随另一个 ref。复制安装的题目包会重新复制。已安装的题目包是只读的，所以有本地修改的题目包会被拒绝更新；`--force` 丢弃这些修改。如果更新后 hc 无法读取 `pack.yaml`，或者题目包改了名，更新会被撤销。更新后 hc 列出新增和删除的题目。你的解答和提交历史按题目的目录名和 slug 关联，所以被删除的题目（或目录被改名的题目）的解答和历史会保留，但不再属于任何题目：hc 会说明这一点，不会删除任何东西。
- **`hc pack remove <name>`** 删除 `packs/<name>/` 和它在 `hc.yaml` 中的条目。你的解答（`solutions/<name>/`）和提交历史保留，重新安装题目包后它们又会出现；`--purge` 会在确认后把它们一并删除（`--yes` 跳过确认）。`local` 不能删除。

### packs 列表

hc 把它安装的每个题目包记录在 `hc.yaml` 中，每次 `hc pack update` 都会更新对应条目：

```yaml
packs:
  - name: leetcode
    source: https://github.com/someone/handcoding-leetcode.git
    ref: v0.3.0        # 跟随的 tag 或分支；不写则跟随默认分支
    commit: 3f2a9c1…   # 已安装的 commit
  - name: mine
    source: /home/me/my-pack   # 复制安装的目录：没有 commit
```

hc 只修改这个列表，`hc.yaml` 的其余部分保留原有的注释和顺序。本地来源记录为绝对路径。

要在别处**复现一个工作区**，把它的 `hc.yaml` 复制到一个新工作区（或者用 git 管理工作区：`hc init` 写入的 `.gitignore` 排除了已安装的题目包），然后不带参数运行 `hc pack install`：它会把列表中缺失的题目包按记录的 commit 安装回来，复制安装的目录则重新复制。

### 检查题目包

`hc pack check` 是题目包的 CI 在发布前运行的检查。在题目包的检出目录中运行它（见[编写题目包](#编写题目包)），或者传入它的目录：`hc pack check ./my-pack`。它检查 `pack.yaml` 合法且协议版本受支持，分组中列出的都是本包的题目，并且每道题都能加载、合法、目录名规范、slug 和 ID 唯一、已就绪（设置了 `judge.compare`）、有用例且每个用例都有期望输出、有参考解，并通过 `hc validate --run`：每个参考解都通过每个用例。因此它需要 Go 和 Python 3.10+，无法运行的参考解也算错误。它为每道题输出一行，最后输出汇总，任何一项失败都以非零状态退出。

## 用编码智能体生成用例

```sh
hc gen 3                                                   # 默认使用 Claude Code
hc gen 3 --agent codex -n 40 -m "more strings of one repeated character"
hc fix two-sum -c 5 -m "answers in a different order should pass"
```

`hc` 在工作区中以无头模式运行智能体（`claude -p` 或 `codex exec`）。启动前，它会安装或更新工作区的 skill（见[智能体 skill](#智能体-skill)）；智能体遵循 **`hc-cases`**，题目定义本身有误时还会用到 **`hc-problem`**。智能体按照 skill：

1. 编写参考解 `reference/solution.py`，必要时再写一个暴力解用来交叉验证；
2. 把测试**输入**写入 `cases.jsonl`，覆盖边界情况、特殊结构和大规模输入；
3. 运行 `hc cases build`，所以期望输出来自运行参考解，而不是模型自己写的；
4. 反复运行 `hc validate --run`，直到每份参考解都通过。

要使用其他智能体，在 `hc.yaml` 中设置它的命令行，`{prompt}` 会被替换为提示词：

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

提示词让智能体读 `.agents/skills/` 下的 skill；命令是 `claude` 时读 `.claude/skills/` 下的。

### 智能体 skill

hc 自带若干 skill，向编码智能体讲解它的题目协议和工作流程：

| Skill | 内容 |
|---|---|
| `hc-problem` | 题目是什么：签名、题面、模板，以及 `hc fetch` 之后要检查什么 |
| `hc-cases` | 题目怎么评判：评判规则、参考解、checker 和测试用例 |
| `hc-pack` | 维护题目包：收录题目、分组、保持 slug 不变、协议版本、CI 和发布 |

工作区得到哪些 skill 取决于它的类型：练习工作区得到 `hc-problem` 和 `hc-cases`，题目包的检出目录（见[编写题目包](#编写题目包)）三个都有。它们被安装到 `.claude/skills/`（供 Claude Code）和 `.agents/skills/`（供 Codex），并在 `AGENTS.md` 的一个区块中列出：新工作区由 `hc init` 安装，任何工作区中 `hc gen`、`hc fix` 和 `hc skill install` 也会安装。由于它们就在工作区里，在工作区中启动的任何 Claude Code 或 Codex 会话都能使用：让它"写一道 hc 题……"、"为这道 hc 题生成用例"或"把这道题加进题目包"即可。skill 引用协议的章节（`hc docs protocol <section>`），而不是重复其内容。

```sh
hc skill list               # 这个工作区得到的 skill；每个 skill 是否安装、是否最新、是否过期或被修改，以及由哪个 hc 版本安装
hc skill install            # 安装或更新工作区的 skill（升级 hc 之后）
hc skill install --force    # 连你修改过的 skill 也一并替换或删除
hc skill install hc-pack    # 按名称安装一个 skill，不论工作区是否得到它
hc skill show hc-cases      # 打印一个 skill
```

hc 会给它安装的每一份副本做标记，记下版本和内容的哈希（skill 目录中的 `.hc-skill.json`），并且只替换或删除这些副本：你修改过的 skill 会保留，除非加上 `--force`；不是 hc 安装的 skill 从不触碰。在 `AGENTS.md` 中，hc 只改写 `<!-- hc:begin -->` 与 `<!-- hc:end -->` 之间的区块，文件的其余部分归你所有。`hc gen` 和 `hc fix` 在启动智能体前，会安装工作区 skill 集合中缺失或过期的 skill。不带名称的 `hc skill install` 则让工作区与它的集合完全一致：还会删除 hc 曾安装、但工作区不该有的 skill（包括按名称安装的）。两者对你修改过的 skill 都只给出警告，既不替换也不删除。

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
- `hc fetch` 可以导入带[结构输入](protocol_zh.md#结构输入)的题目（带环链表、相交链表、二叉树的最近公共祖先这类节点引用），结构由 LeetCode 的元数据（描述它的测试数据）与初始代码（真正的签名）之间的差异推出：初始代码不接收的测试数据成为隐藏输入，初始代码以节点接收的值成为 `ref`，测试数据中没有的节点参数成为 `copy`，`pos` 形成 `cycle`，160 的 `skipA` 和 `skipB` 形成 `join`，返回节点的题目得到 `returns_ref`。示例会对应到新的输入上。不是普通 JSON 的示例输出（如 142 的 "tail connects to node index 1"）留给 `hc gen` 补写。回归测试集中的 141、142、160、236、237 和 1379 都属于这类题目。
- `hc fetch` 可以导入使用 [LeetCode 的 `Node` 类型](protocol_zh.md#leetcode-的-node-类型)、`NestedInteger`、`Employee` 的题目，签名取自 LeetCode 的 Go 初始代码。`returns_copy`（克隆图）留给 `hc gen` 设置；扁平化嵌套列表迭代器这类迭代器题目的示例只给出输入、没有给出调用序列，也留给 `hc gen` 补写。
- 把一个值编码再解码回来的题目（297、449、535）按[编解码往返题](protocol_zh.md#编解码往返题)评测：只比较解码得到的值，所以对编码本身的要求（449 的"尽可能紧凑"）不会检查。`hc fetch` 可以导入这类题目，编码器、解码器以及二者是否共用一个实例取自 LeetCode 初始代码注释中的调用方式。回归测试集中的 297 和 535 分别展示了两种情况。
- 会统计运行时间，但不限制内存。
- 无法获取 LeetCode 会员题目。
- 合法答案很多、无法全部列出的题目（"返回任意一个合法答案"）和随机题由[判题程序](protocol_zh.md#判题程序)评测，判题程序由 `hc gen` 编写，回归测试集中的 5、108、210、324、384 和 528 都带有判题程序。判题程序用 Python 编写，所以即使用 Go 作答，这些题目也需要 Python 3.10 或更新版本；hc 管理的虚拟环境会提供它。
