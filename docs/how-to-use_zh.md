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

`hc init` 创建一个空的工作区，默认位置是 `~/handcoding`，并提示安装官方 LeetCode 题目包 [leetcode.hc](https://github.com/MorePeanuts/leetcode.hc)（见[题目包](#题目包)）：

```
~/handcoding/
  hc.yaml                                  配置文件，以及已安装题目包的列表
  packs/local/problems/0001-two-sum/       你自己的题目（见题目协议）
  packs/<name>/                            已安装的题目包（只读）
  solutions/local/go/0001-two-sum/         你的代码：solution.go，以及自动生成的 go.mod 和 zz_hc_driver.go
  solutions/local/python/0001-two-sum/     你的代码：solution.py，以及自动生成的 hc_runtime.py 和 _hc_runner.py
  .hc/hc.db                                提交历史（SQLite）
  .hc/venv/                                hc 管理的 Python 虚拟环境
  .hc/cache/                               供编码智能体使用的 Go 和 uv 缓存，以及生成用例（cases/）、Go 程序（build/）和参考解用时（baseline/）
  pyrightconfig.json                       让 pyright 使用 .hc/venv
```

`hc init --lang python` 把默认解题语言设为 Python（即 `hc.yaml` 中的 `language`，默认为 `go`），并立即创建下文介绍的 Python 环境。如果创建失败，例如既没有 uv 也没有 Python 3.10+，`hc init` 会给出警告，但仍然创建工作区。

`hc` 按以下顺序查找工作区：`-w <dir>`、`$HC_WORKSPACE`、包含 `hc.yaml` 或 `pack.yaml`（题目包的检出目录，见[编写题目包](#编写题目包)）的最近一级父目录、`~/handcoding`。所以在任何目录下都可以运行它。

### 工作区中的题目包

工作区里的题目按[题目包](protocol_zh.md#题目包)组织，每个题目包是 `packs/` 下的一个目录：

- `packs/local/` 存放你自己的题目，也就是 `hc fetch`、`hc new` 和 `hc import` 添加的题目。它没有 `pack.yaml`（`local` 是保留名称），可以用所有语言作答。它的题面使用 `hc.yaml` 中 `locales` 设置的[语言区域](#多语言题面)，默认为英文（`[en]`）。
- 其余每个目录都是一个已安装的题目包：一个 `name` 与目录名相同的 `pack.yaml`，加上它的 `problems/`。已安装的题目包是**只读**的：`hc gen`、`hc fix` 和 `hc cases add|rm|edit|build` 拒绝修改它们，因为更新题目包会覆盖这些修改。`pack.yaml` 列出了 `languages` 的题目包只能用这些语言作答：`hc start -l` 拒绝其他语言；如果你的默认语言不在其中，hc 会改用题目包的第一种语言并给出提示。`hc pack install` 负责安装题目包（见[题目包](#题目包)）；手动复制到 `packs/` 下的题目包也能用，`hc pack list` 会把它显示为 unmanaged（未托管）。

hc 无法读取的题目包（没有 `pack.yaml`、`pack.yaml` 不合法，或为更新的协议编写，此时会提示升级 hc）会由 `hc validate` 和题目列表报告，它的题目不会被加载；工作区的其余部分照常工作。

解答存放在 `solutions/<pack>/<lang>/<dir>/`，提交历史记录每次提交的题目包和 slug，所以两个题目包里相同的 slug 是两道不同的题。

旧版 hc 创建的工作区把题目放在 `problems/` 而不是 `packs/` 下。hc 遇到这种工作区会停下并说明：用 `hc init <dir>` 新建一个工作区，再把你的题目移到它的 `packs/local/problems/` 下。解答和提交历史不会迁移。

每道 Go 题都是一个独立的 module，驱动文件就放在 `solution.go` 旁边。因此 gopls、`go build` 和 Delve 断点都直接作用在你自己的文件上。自动生成的文件每次运行前都会重写，不要手动修改。`go.mod` 跟随本机的 Go：其中的 `go` 行就是本机工具链的版本（Go 1.27.2 对应 `go 1.27`），所以你的解答可以使用它支持的所有特性。hc 用 `GOTOOLCHAIN=local` 构建，不会下载其他版本的工具链。构建出的程序按其源文件的内容保存在 `.hc/cache/build/go/` 中，所以代码没有改动时再次运行不会重新构建；在 macOS 上，这还省去了系统在每个新程序首次启动时做的检查。最多保留最近使用的 32 个。

Python 解答在 hc 管理的虚拟环境 `.hc/venv` 中运行。第一次有命令需要 Python 时（`hc init --lang python`、`hc start`、`test`、`submit`、`debug`、`cases build`、`validate --run`），hc 会创建它：

1. 如果装了 [uv](https://docs.astral.sh/uv/)，用 `uv venv --python '>=3.10'` 创建；本机没有合适的 Python 时，uv 会自动下载一个；
2. 否则从 PATH 上的 `python3`、`python` 和 `python3.N` 中选出最新的 Python 3.10+，用 `-m venv` 创建。在 Debian 和 Ubuntu 上这需要安装 `python3-venv` 包。

在题目包的 checkout 中（见[编写题目包](packs_zh.md)），环境改用 Python 3.10，即协议支持的最旧版本，题目包的参考解、判题程序和生成器都必须能在它上面运行：用 `uv venv --python 3.10` 创建，没有 uv 时用 PATH 上最新的 `python3.10`。这样 `hc validate --run` 和 `hc pack check` 与题目包的 CI 使用同一个 Python。已有的环境版本不对时会重新创建；找不到 Python 3.10 时（没有 uv，PATH 上也没有），hc 使用最新的版本并给出警告。

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
  .claude/  .agents/  AGENTS.md  CLAUDE.md   智能体技能与说明，在某个命令安装之后出现
```

没有 `hc.yaml` 时 hc 使用默认设置，并且从不在检出目录中创建它。检出目录的 `locales` 来自 `pack.yaml`，所以其中的 `hc.yaml` 不得设置它，就像不得列出 `packs` 一样。hc 无法读取的 `pack.yaml`（例如为更新的协议编写）不会妨碍工作区打开：`hc validate` 会指出问题所在。hc 添加的这些内容属于你而不属于题目包，所以不要把它们提交到仓库：

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

- 在题目上按 `enter` 打开题目：显示题面，根据模板创建解答文件，并把它的绝对路径**复制到剪贴板**。在编辑器里 `:e <粘贴>` 就可以开始写。如果只想看题面、不打算开始做，按 `v`：在 `e`、`t`、`y` 等操作需要解答文件之前，不会创建或复制任何东西。
- 在题面页面可以测试、提交、调试和查看历史。结果显示在界面中，程序自己的输出单独列在下方。
- 调试器会接管终端：启动前清屏，退出后回到界面。如果上次运行有失败的用例，按 `d` 调试的就是那个用例。
- 题面图片直接画在终端里。kitty 和 Ghostty 显示原图；其他真彩色终端用块字符近似显示，会损失细节，可以按 `o` 用看图软件打开原图。显示方式见 [`images`](#配置)。
- 题面配色跟随终端背景色。如果颜色不对，在 `hc.yaml` 中设置 `theme`。
- 列表一次显示一个题目包。有多个题目包时，顶部的标签栏列出每个题目包及其进度（已通过/总数），用 `tab` / `shift+tab` 切换题目包；终端较窄时，标签栏会滚动，让当前题目包保持可见。没有题目的题目包（例如还没添加题目时的 `local`）不显示标签，除非所有题目包都还没有题目。
- 按 `m` 收藏想回头再做的题目，再按一次 `m` 取消收藏。收藏的题目在列表和题面标题中显示 `★`，`f` 筛选可以只列出收藏的题目。收藏保存在 `.hc/hc.db` 中，放弃题目的解答后仍会保留。
- 按 `c` 选择题目包中的一个分组，或者全部题目，每一项都显示进度。分组按分组中的顺序列出题目。搜索和筛选作用于当前显示的题目包和分组。
- 有多个包含题目的题目包时，`hc` 会先打开题目包选择页，列出这些题目包及其进度，光标停在上次所在的题目包上。按 `enter` 打开题目包的列表，在列表中按 `esc` 回到选择页。只有一个这样的题目包时，`hc` 直接打开它的列表。
- `hc --pack <name>`（或 `hc list --pack <name>`）跳过选择页，直接打开该题目包（即使它是空的），`--group <name>` 打开该分组（只有一个题目包使用的分组名不需要 `--pack`）。每个题目包都会打开上次选择的分组。
- 题目包的题面有[多种语言](#多语言题面)时，按 `i` 切换题面语言：列表中的标题和详情页的题面都会切换。列表顶部和题面的标题行会显示当前使用的语言区域。

| 按键 | 操作 | 按键 | 操作 |
|---|---|---|---|
| `enter` | 打开题目（并复制解答路径） | `e` | 在编辑器中打开解答 |
| `t` | 用示例用例测试 | `s` | 提交（全部用例） |
| `d` | 调试（如有，调试上次失败的用例） | `h` | 提交历史：代码、判题详情、恢复、删除 |
| `y` | 再次复制解答路径 | `L` | 切换语言 |
| `r` | 重置为模板 | `D` | 放弃：删除解答及其历史，回到未开始状态 |
| `/` | 搜索 | `f` | 筛选：全部 / 未通过 / 已通过 / 已收藏 |
| `tab` / `shift+tab` | 下一个 / 上一个题目包 | `c` | 选择题目包中的分组 |
| `v` | 只查看题面 | `o` | 打开题面中的图片（题面页面） |
| `i` | 切换题面语言 | `esc` / `q` | 返回（在列表页按 `esc` 回到题目包选择页）；在列表页按 `q` 退出 |
| `m` | 收藏或取消收藏题目（列表中显示 `★`） | `ctrl+c` | 退出 |

## 命令

```
练习          list  start  edit  show  test  submit  debug  reset  discard  history  leetcode
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
hc test two-sum --all --file draft.py               # 运行另一个解答文件，而不是你的解答
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

报告会列出每个用例在你的代码中花费的时间：函数题是函数调用本身，设计题是构造函数和每次方法调用，不含参数的解码和结果的编码。末尾的总用时是这些时间之和，不含启动程序和读取用例的时间。

`--file` 运行放在任意位置的解答文件，而不是 `solutions/` 中的那份，便于比较不同的草稿，或者检查一份慢速解会不会超出[时间限制](#时间限制)（`--file` 指向参考解本身的文件即可看到它的用时）。它的语言由扩展名决定，除非用 `-l` 另行指定。这个文件会被复制到单独的目录中运行，所以不能依赖旁边的其他文件。

### 时间限制

和 LeetCode 一样，正确但太慢的解答判为超时（Time Limit Exceeded）。hc 以题目在你所用语言中的参考解为准评判速度，并在你的机器上计时（见[协议中的时间限制](protocol_zh.md#时间限制)）：一次运行的总用时最多可以是参考解在相同用例上总用时的 **40 倍**，参考解的用时按不少于 **1.25ms** 计，所以限制不会低于 50ms：低于这个量级，计时主要是噪声。这两个值仍可能调整。运行停在总用时超过限制的那个用例上：这个用例判为超时（即使它永不返回，也会被终止），其后的用例为 Not Run。

- 40 倍给同一个算法的不同写法留出了余地，比如参考解用数组而你用 map，但在题目约束允许的最大输入上容不下更差的复杂度。较慢的解答会不会失败取决于题目的用例：有些题目在约束范围内拉不开两种复杂度的差距，这时较慢的解答会通过，在 LeetCode 上也一样。
- 一道题第一次运行时，hc 会给参考解计时：每个用例运行三次，取最快的一次，保存在 `.hc/cache/baseline/<pack>/<problem>/<language>.json`。hc 会在 stderr 上提示（"Timing the Go reference…"），之后的运行无论选哪些用例都复用这些时间。hc、你的工具链、`problem.yaml` 或参考解改变时会重新测量。
- 题目没有你所用语言的参考解时不作评判（见[就绪](protocol_zh.md#就绪)），所以也没有时间限制。参考解、`hc cases build` 和 `hc validate --run` 从不受限，`hc debug` 也一样。
- `hc.yaml` 中的 `timeout` 是另一个限制，针对整次运行的墙钟时间：它用来防止解答卡死。它也可能先于时间限制生效：参考解在本次运行用例上的总用时超过它的四十分之一时（默认 10s 下即 250ms，Python 参考解在大用例较多的题目上可能如此），慢得多的解答会先用完 `timeout`。判定结果相同，都是超时（Time Limit Exceeded），只是要等满整个 `timeout`。

## 获取题目

### 从 LeetCode 获取

```sh
hc fetch 3 146 lru-cache    # 按编号、slug 或 URL，从 leetcode.com 获取
hc fetch --site cn 1        # 从 leetcode.cn 获取，它还有中文题面
hc fetch https://leetcode.cn/problems/two-sum/description/   # 任意题目 URL，站点由 URL 决定
hc fetch --site cn "LCR 001"                                  # leetcode.cn 的 LCR/LCP 等编号
hc gen 146                  # 让题目就绪：生成测试用例并决定判题规则
```

`hc fetch` 会下载题面及其图片、函数签名、Go 和 Python 的初始代码，以及示例用例。能从题面中解析出期望输出时，也会一并写入。

它按目标题目包的语言区域写入题面：你自己的题目由 `hc.yaml` 中的 `locales` 决定（见[多语言题面](#多语言题面)）。两个站点都有英文题面；只有 leetcode.cn 有中文题面和中文标题：

| `hc.yaml` 中的 `locales` | leetcode.com | leetcode.cn |
|---|---|---|
| `[en]`（默认） | `description.md`：英文 | `description.md`：英文 |
| `[en, zh-CN]` | `description.md`：英文；没有中文题面，并给出提示 | 两者都有：英文写入 `description.md`，中文写入 `description.zh-CN.md`，以及两个标题 |
| `[zh-CN]` | 拒绝：leetcode.com 没有中文题面 | `description.md`：中文 |

从另一个站点再次拉取同一道题，会补上它缺少的题面，不需要 `--force`，也不会改动其他内容：先 `hc fetch two-sum`，再 `hc fetch --site cn two-sum`，题目就有了两种题面。题面共用 `assets/`，两者都用到的图片只保存一份。只有中文的题目（LCR、LCP）无法拉取到默认语言区域为 `en` 的题目包。

刚拉取的题目**还不能提交**。LeetCode 只公开示例，也不公开判题方式：顺序是否重要、是否有多个正确答案。在判题规则确定之前，`hc test` 会把你的输出和示例答案并排列出，但不给结论；`hc submit` 会拒绝提交。`hc gen` 会生成完整的测试用例，并根据题面决定判题规则，内置规则都不适用时编写判题程序；你也可以自己设置 `judge.compare`（见协议的[比较方式](protocol_zh.md#比较方式)）。题目列表会在状态列用 `?` 标记这类题目，已通过的题目在这一列显示 `✔`，尝试过的显示 `●`。

题目只在有参考解的语言中评判：Go 解答需要 `reference/solution.go`，Python 解答需要 `reference/solution.py`（协议的[就绪](protocol_zh.md#就绪)）。没有对应的参考解时，`hc test` 只列出你的输出、不给结论，`hc submit` 拒绝提交并指出缺少的文件，题目列表对你所用的语言显示 `?`。`hc gen` 会为题目包支持的每种语言编写参考解。

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
hc rm my-problem                    # 删除题目（加 --purge 时连同你的解答和提交历史）
```

`hc fetch`、`hc new` 和 `hc import` 都写入 `packs/local/problems/`。题目的格式，从 `problem.yaml` 到参考解，由[题目协议](protocol_zh.md)规定；`hc new` 会生成一个待填写的骨架，hc 的回归测试集（仓库中的 [`testdata/regression`](../testdata/regression)）则包含覆盖每项功能的完整例子。

- `hc cases build` 运行一个参考解：有 Go 参考解时用 Go，否则用 Python；`-l` 可以指定。判题方式为 `any_of` 的题目会被拒绝，因为它的 `expected` 要列出所有可接受的答案，只能手写。
- `hc validate` 把每一处违反协议[合法性规则](protocol_zh.md#合法性规则)的地方报为错误，并注明用例编号和字段。加上 `--run` 时，它还会在每个用例上运行每个参考解，有判题程序时交给判题程序判定。以下情况给出警告：判题规则未确定（没有 `judge.compare`）、没有用例、有用例缺少 `expected`、有生成用例缺少摘要、有存储的用例超过 100 KB、没有用例标记为示例、没有参考解或缺少题目包所支持的某种语言的参考解，以及题面引用的图片不存在。`hc test`、`hc submit` 和 `hc debug` 拒绝运行违反这些规则的用例，并指出是哪一个。
- `hc import` 只复制协议定义的条目（其余内容，如编辑器文件、`solutions/` 目录，不会被导入）；如果你自己的题目中已有相同的 slug 或编号，会拒绝导入（`--force` 会替换 slug 相同的题目）。slug 和编号只需在一个题目包内唯一，所以导入一道已安装题目包里也有的题目没有问题。题面和标题与你自己题目的语言区域不符的题目也会被拒绝，就像 `hc validate` 会报告的那样：缺少某个语言区域的题面，或者有其他语言区域的题面。
- `hc rm` 删除题目的目录；在题目包检出目录中，还会把它的 slug 从 `pack.yaml` 的各个分组中移除，并保留该文件的注释和格式。它会先询问确认（`--yes` 跳过询问），并且拒绝修改已安装的题目包。你对这道题的解答、提交历史和收藏会保留，以后再添加同一 slug 的题目时会重新对应上；加 `--purge` 则一并删除。
- 不带参数的 `hc validate` 还会检查每个已安装题目包的 `pack.yaml`，以及它的分组是否都指向本题目包的题目。每道题的目录名、slug 和编号都会与所在题目包的其他题目对照检查。

hc 内置了与自身版本对应的协议（英文版），离线也能查阅：

```sh
hc docs protocol                    # 完整协议
hc docs protocol --list             # 列出各节
hc docs protocol structure-inputs   # 某一节，包括其下的小节
```

每一节用英文版标题在 GitHub 上的锚点命名，如 `protocol.md#structure-inputs`。在终端中输出会像题面一样渲染；通过管道输出或加上 `--raw` 时输出 Markdown 原文。

### 多语言题面

题目包可以用多种语言提供题面，即它的[语言区域](protocol_zh.md#语言区域)：在 `pack.yaml` 中写 `locales: [en, zh-CN]`。第一个是题目包的默认语言区域，也就是 `description.md` 和 `title` 的语言；其他语言区域的题面在 `description.<locale>.md` 中，标题在 `problem.yaml` 的 `titles` 中。题目包有哪些语言区域、哪个排在第一，由题目包的作者决定。你自己的题目，作者就是你，由 `hc.yaml` 中的 `locales` 设置，未设置时为 `[en]`：

```yaml
locales: [en, zh-CN]   # 你自己的题目：英文优先，也有中文
```

- 在列表中按 `i` 切换题目包的语言区域，hc 会在 `.hc/` 中为每个题目包记住这个选择。第一次打开的题目包显示其默认语言区域。
- `hc show <problem> --locale zh-CN` 显示某个语言区域的题面；不带 `--locale` 时，显示你上次在列表中选择的那个。
- 题目缺少所选语言区域的题面时（只有不合法的题目包才会这样），hc 显示默认语言区域的题面，并加以说明。
- `hc validate` 会报告缺少某个语言区域的题面或标题的题目，以及使用了题目包未声明的语言区域的题面或标题。`hc pack check` 也会因此失败。
- hc 不翻译任何内容。`hc fetch` 写入 LeetCode 提供的题面（见[上文](#从-leetcode-获取)）；你自己的题目，每种语言的题面都要自己写。`hc gen` 和 `hc fix` 只读取默认语言区域的题面，也从不写入题面。

### 测试用例

```sh
hc cases list 3                     # 查看用例
hc cases add 3 -i '{"s":"abba"}'    # 追加一个用例（有参考解时会算出期望输出）
hc cases add 3 --generate long --args '{"n":100000}'   # 追加一个生成用例
hc cases edit 3                     # 在编辑器中编辑 cases.jsonl
hc cases build 3                    # 用参考解补全缺失的期望输出（--force：全部重算）
```

### 生成用例

大规模输入，也就是能抓出复杂度不对的解答的那种输入，不该写进 `cases.jsonl`：几个这样的用例就能让一道题有几 MB 大，而它们的每次改动都会永远留在题目包的 git 历史里。改为编写[生成器](protocol_zh.md#生成器)：题目的 `generator.py` 中一个返回输入的函数，再添加一个调用它的用例：

```python
# generator.py
def descending(rng, n):
    return {"nums": list(range(n, 0, -1))}      # 手工设计的最坏情况：不用 rng

def distinct(rng, n):
    return {"nums": rng.sample(range(-10**9, 10**9 + 1), n)}
```

```json
{"generate":{"name":"descending","args":{"n":100000}},"note":"descending, quadratic for insertion sort"}
{"generate":{"name":"distinct","args":{"n":100000},"seed":7}}
```

- `hc cases build` 重新运行生成器，并写入每个用例的 `digest`，即它构造出的输入的哈希。对摘要新增或改变了的用例（`--force` 时为全部），它以所运行的那份参考解的输出为期望输出评判其他每份参考解，所以 Go 和 Python 的参考解必须一致。生成用例不存储期望输出。
- `hc test`、`hc submit` 和 `hc debug` 会构造它们要运行的生成用例：无论你用什么语言解题，生成器都由 Python 运行；期望输出由你所用语言的参考解（没有时用另一份）计算，然后照常评判。结果保存在 `.hc/cache/cases/<pack>/<problem>/<language>.json`，当 hc、`problem.yaml`、`generator.py`、参考解或生成用例改变时会重新构造。
- 生成器失败、构造出的输入与摘要不符或不合法，以及参考解在生成的输入上失败，都是题目的错误，而不是对你的解答的判题结果：运行会停止，并指出文件和用例。`hc validate --run` 和 `hc pack check` 总是重新构造这些用例，从不读缓存，以便发现不确定的生成器。
- `hc test --generate <name> --args '{…}' --seed N` 用 `generator.py` 中的函数构造一个自定义输入，像 `-i` 那样运行，但不会往 `cases.jsonl` 添加用例，也不改动缓存；期望输出由参考解计算。用这种方式分别在 `n` 和 `n/2` 上计时，可以看出解答的用时如何增长。
- `hc cases list` 把生成用例显示为它的调用，例如 `distinct(n=100000) seed 7`；报告中失败的生成用例显示为调用加上截断的输入。`hc cases list --expand` 会运行 `generator.py`，显示它当前生成的输入（加上 `--json` 时，输出为存储用例那样的行，便于脚本检查），不检查摘要。
- `hc validate` 对超过 100 KB 的存储用例给出警告，`hc pack check` 则判为失败。

## 题目包

[题目包](protocol_zh.md#题目包)是一个带有 `pack.yaml` 的现成题目目录，通常是一个 git 仓库。hc 调用 PATH 上的 `git` 来安装和更新题目包。

```sh
hc pack install https://github.com/MorePeanuts/leetcode.hc.git   # 把题目包克隆到 packs/<name>/
hc pack install ../my-pack --ref v1.2.0                          # 本地 git 仓库，指定 tag 或分支
hc pack list                                # 题目包、版本、commit、来源和你的进度
hc pack update                              # 更新所有题目包（也可以指定名字）
hc pack update leetcode --ref main          # 改为跟随另一个 tag 或分支
hc pack remove leetcode                     # 删除题目包；你的解答和提交历史保留
hc pack remove leetcode --purge             # ……连同它们一起删除
```

> [!WARNING]
> 安装题目包意味着信任它的代码：测试、提交或校验时，它的参考解、checker 和生成器会在你的机器上运行。只从你信任的来源安装题目包。

- **`hc pack install <source>`** 接受 git URL 或目录。git URL 以部分克隆（`--filter=blob:none`）的方式克隆：hc 下载所有 commit，但只下载它检出的 commit 中的文件，所以无论题目包的文件曾经多大，它的历史都不占多少空间。本身是 git 仓库的目录会被完整克隆；其他包含 `pack.yaml` 的目录会被复制（只复制 `pack.yaml` 和 `problems/`）。`--ref` 安装 git 来源的某个 tag 或分支，而不是默认分支。题目包安装到 `packs/<name>/`，名字取自它的 `pack.yaml`。hc 拒绝不合法的 `pack.yaml`、为它不支持的协议版本编写的 `pack.yaml`（并说明该升级 hc 还是题目包），以及与 `packs/` 中已有题目包重名的题目包。加载失败的题目会被报告，但不会阻止安装。
- **`hc pack list`** 显示每个题目包的名字、版本、题目数、进度（有通过提交的题目数）、已安装的 commit 和来源。`packs/` 中 `hc.yaml` 没有列出的目录（例如手动复制进来的）标为 *unmanaged*（未托管）；`hc.yaml` 列出了但 `packs/` 中没有的题目包标为 *missing*（缺失）。
- **`hc pack update [name]...`** 拉取每个题目包，并检出它的 ref 的最新 commit：分支会前进，tag 保持不动（除非上游移动了它），`--ref` 改为跟随另一个 ref。安装时没有指定 `--ref` 的题目包跟随源仓库的默认分支，源仓库给默认分支改名后也一样。复制安装的题目包会重新复制。已安装的题目包是只读的，所以有本地修改的题目包会被拒绝更新；`--force` 丢弃这些修改。如果更新后 hc 无法读取 `pack.yaml`，或者题目包改了名，更新会被撤销。更新后 hc 列出新增和删除的题目。你的解答和提交历史按题目的目录名和 slug 关联，所以被删除的题目（或目录被改名的题目）的解答和历史会保留，但不再属于任何题目：hc 会说明这一点，不会删除任何东西。
- **`hc pack remove <name>`** 删除 `packs/<name>/` 和它在 `hc.yaml` 中的条目。你的解答（`solutions/<name>/`）和提交历史保留，重新安装题目包后它们又会出现；`--purge` 会在确认后把它们一并删除（`--yes` 跳过确认）。`local` 不能删除。
- 同一个工作区里，`hc pack install`、`update` 和 `remove` 一次只能运行一个：一个正在运行时，另一个会拒绝启动并给出说明，而不会撤销前者对 `packs/` 和 `hc.yaml` 的修改。

### packs 列表

hc 把它安装的每个题目包记录在 `hc.yaml` 中，每次 `hc pack update` 都会更新对应条目：

```yaml
packs:
  - name: leetcode
    source: https://github.com/MorePeanuts/leetcode.hc.git
    ref: v0.3.0        # 跟随的 tag 或分支；不写则跟随默认分支
    commit: 3f2a9c1…   # 已安装的 commit
  - name: mine
    source: /home/me/my-pack   # 复制安装的目录：没有 commit
```

hc 只修改这个列表，`hc.yaml` 的其余部分保留原有的注释和顺序。本地来源记录为绝对路径。

要在别处**复现一个工作区**，把它的 `hc.yaml` 复制到一个新工作区（或者用 git 管理工作区：`hc init` 写入的 `.gitignore` 排除了已安装的题目包），然后不带参数运行 `hc pack install`：它会把列表中缺失的题目包按记录的 commit 安装回来，复制安装的目录则重新复制。

### 检查题目包

`hc pack check` 是题目包的 CI 在发布前运行的检查。在题目包的检出目录中运行它（见[编写题目包](#编写题目包)），或者传入它的目录：`hc pack check ./my-pack`。它检查 `pack.yaml` 合法且协议版本受支持，分组中列出的都是本包的题目，并且每道题都能加载、合法、目录名规范、slug 和 ID 唯一、已就绪（设置了 `judge.compare`）、有用例且每个用例都有期望输出（生成用例则有摘要）、没有超过 100 KB 的用例、对题目包 `languages` 中的每种语言都有参考解，并通过 `hc validate --run`：每个参考解都通过每个用例，生成用例重新构造。因此它需要 Go 和 Python 3.10+，无法运行的参考解也算错误。它为每道题输出一行，最后输出汇总，任何一项失败都以非零状态退出。

## 用编码智能体生成用例

```sh
hc gen 3                                                   # 默认使用 Claude Code
hc gen 3 --agent codex -n 40 -m "more strings of one repeated character"
hc fix two-sum -c 5 -m "answers in a different order should pass"
```

`hc` 在工作区中以无头模式运行智能体（`claude -p` 或 `codex exec`）。启动前，它会安装或更新工作区的 skill（见[智能体 skill](#智能体-skill)）；智能体遵循 **`hc-cases`**，题目定义本身有误时还会用到 **`hc-problem`**。智能体按照 skill：

1. 为题目包支持的每种语言编写参考解（`reference/solution.py`、`reference/solution.go`），必要时再写一个暴力解用来交叉验证；
2. 把测试**输入**写入 `cases.jsonl`，覆盖边界情况和特殊结构，大规模输入写成[生成用例](#生成用例)；
3. 运行 `hc cases build`，所以期望输出来自运行参考解，而不是模型自己写的；
4. 反复运行 `hc validate --run`，直到每份参考解都通过。

预设会在沙箱中运行智能体，它执行的命令只能写入工作区和临时目录（Claude Code 的沙箱，Codex 的 `workspace-write`）。因此 hc 会在智能体启动前建好 `.hc/venv` 中的 Python 环境，并为智能体（无论是否用预设）把缓存放在工作区内：`GOCACHE` 为 `.hc/cache/go-build`，`UV_CACHE_DIR` 为 `.hc/cache/uv`。所以智能体第一次构建 Go 时会慢一些。

智能体工作时，hc 会显示它运行的每条命令和写入的每个文件，结束后显示它的最终报告、运行时长（Claude Code 还会显示轮数和费用）以及执行轨迹的位置。**每次运行都会记录日志**，保存在 `.hc/agent/<pack>/<slug>/<时间>-<gen|fix>.jsonl`，每道题保留最近 10 份。日志是 JSON 行：

- 第一行是 `{"type": "start", ...}`：hc 的版本、任务、题目、智能体及其命令行，以及提示词；
- 之后智能体输出的每一行对应一行日志，带有 hc 读到它的时间：`{"type": "stdout", "event": ...}` 是预设 JSON 输出中的一个事件（Claude Code 的 `--output-format stream-json`、`codex exec --json`），`"text"` 则是其他任何一行，标准错误（`"stderr"`）也一样；
- 最后一行是 `{"type": "end", ...}`：运行时长、退出码，以及出错时的错误信息。

读日志就能看到智能体做了什么、时间花在哪里，比如用 `jq`。智能体自己也有记录，但在别处，也看不出属于哪次 hc 运行。

要使用其他智能体，在 `hc.yaml` 中设置它的命令行，`{prompt}` 会被替换为提示词：

```yaml
agent:
  command: ["my-agent", "--yes", "{prompt}"]
```

提示词让智能体读 `.agents/skills/` 下的 skill；命令是 `claude` 时读 `.claude/skills/` 下的。这类命令的输出，hc 按文本显示和记录。

### 智能体 skill

hc 自带若干 skill，向编码智能体讲解它的题目协议和工作流程：

| Skill | 内容 |
|---|---|
| `hc-problem` | 题目是什么：签名、题面、模板，以及 `hc fetch` 之后要检查什么 |
| `hc-cases` | 题目怎么评判：评判规则、参考解、checker 和测试用例 |
| `hc-pack` | 维护题目包：收录题目、分组、保持 slug 不变、协议版本、CI 和发布 |

工作区得到哪些 skill 取决于它的类型：练习工作区得到 `hc-problem` 和 `hc-cases`，题目包的检出目录（见[编写题目包](#编写题目包)）三个都有。它们被安装到 `.claude/skills/`（供 Claude Code）和 `.agents/skills/`（供 Codex），并在 `AGENTS.md` 的一个区块中列出，这个区块还告诉智能体把临时文件放在 `.hc/scratch/<slug>/` 中，每道题一个目录，这样同时处理不同题目的智能体（例如并行运行的几个 `hc gen`）不会互相覆盖文件：新工作区由 `hc init` 安装，任何工作区中 `hc gen`、`hc fix` 和 `hc skill install` 也会安装。Claude Code 本身不读取 `AGENTS.md`，所以工作区没有 `CLAUDE.md` 时，hc 还会创建一个导入它的 `CLAUDE.md`（`@AGENTS.md`）；你已有的 `CLAUDE.md` 归你所有，hc 从不触碰。由于它们就在工作区里，在工作区中启动的任何 Claude Code 或 Codex 会话都能使用：让它"写一道 hc 题……"、"为这道 hc 题生成用例"或"把这道题加进题目包"即可。skill 引用协议的章节（`hc docs protocol <section>`），而不是重复其内容。

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

## 在 LeetCode 上运行

`hc leetcode` 在 LeetCode 上运行解答，或把它提交到你自己的账号，相当于题目页面上的"运行"和"提交"按钮。它只用于来自 LeetCode 的题目（用 `hc fetch` 获取，或来自官方题目包等题目包）：站点和 slug 取自 `problem.yaml` 的 `source`，`--site` 可以换成另一个站点。

```sh
hc leetcode login --site cn          # 每个站点一次：在浏览器中授权 hc
hc leetcode status                   # hc 登录了哪些站点，以谁的身份
hc leetcode test two-sum             # 在 LeetCode 的示例测试用例上运行
hc leetcode test two-sum -i $'[3,3]\n6'   # 或者用自己的输入，采用 LeetCode 的格式
hc leetcode submit two-sum -l python # 提交到你的账号
hc leetcode logout                   # 忘掉所有站点的会话（--site：只忘掉一个）
```

**登录。** LeetCode 登录需要验证码，所以 hc 不用密码，而是使用浏览器的会话。默认情况下，`hc leetcode login` 会像 LeetCode 的 VS Code 插件那样，在浏览器中打开该站点的授权页面（尚未登录的话，先在那里登录）。随后 LeetCode 把浏览器的 cookie 发送到 hc 在 `127.0.0.1` 上监听的端口，页面会提示 hc 已经登录。cookie 是通过这个 URL 传递的，所以它会留在浏览器的历史记录中。在没有桌面环境的机器上，或者通过 SSH 登录时，先转发端口，再在你自己的机器上打开 hc 打印的页面：

```sh
ssh -L 8765:127.0.0.1:8765 server     # 在你的机器上
hc leetcode login --port 8765         # 在服务器上
```

授权页面并不是 LeetCode 公开文档中的 API。如果它不再可用，就用 `--paste` 登录（标准输入不是终端时也会这样）：在浏览器中登录该站点，打开开发者工具（F12），然后复制下面任意一种：

- `LEETCODE_SESSION` 这个 cookie 的值，位于 Application（Chrome、Edge）或 Storage（Firefox、Safari）→ Cookies；
- 或者在 Network 中任选一个发往该站点的请求（列表为空时刷新页面）：右键 → Copy → Copy as cURL，或者复制它的 `Cookie` 请求头。

在 `hc leetcode login --paste` 询问时粘贴（粘贴时不会显示任何内容），或者通过管道传入（macOS 上是 `pbpaste | hc leetcode login --site cn`）。hc 从粘贴的内容中找出 `LEETCODE_SESSION`。

无论哪种方式，hc 都会向 LeetCode 验证会话，然后保存到用户配置目录（macOS 上是 `~/Library/Application Support`，Linux 上是 `~/.config`）下的 `hc/leetcode.json`，只有你自己可读，绝不会放在工作区里。会话的有效期是几周；LeetCode 拒绝它时，重新登录即可。`hc leetcode status` 向 LeetCode 逐个验证已保存的会话，显示它登录的用户名，或者 LeetCode 已不再接受它；只有 hc 已登录（`--site` 指定的站点，或者任意一个站点）且每个已保存的会话都有效时，退出码才是 0，否则是 1。`hc leetcode logout` 忘掉所有站点的会话，或者 `--site` 指定的那个，但只是 hc 不再保存：会话在过期之前在 LeetCode 那边仍然有效，要让它失效，需要在浏览器中退出登录。

**代码。** hc 发送的是 `hc test` 会运行的那份解答（`-l` 的用法相同），并转换成 LeetCode 的代码：Python 去掉从 `hc_runtime` 的导入（LeetCode 自己定义了 `ListNode`、`TreeNode` 和 `Node`），Go 去掉 `package main`。Python 的函数题在 LeetCode 上必须是 `class Solution` 的方法；hc 也接受模块级函数，但 LeetCode 不接受。

**结果。** `test` 显示每个示例的输出、LeetCode 的期望答案以及打印的内容。`submit` 显示判定结果、通过的测试用例数，以及运行时间和内存及其百分位。提交失败时，会显示失败的那个用例、你的输出和期望输出；如果题目的输入就是 LeetCode 的原样输入（没有[结构输入](protocol_zh.md#结构输入)和 LeetCode 自有类型的函数题），还会打印添加这个用例的 `hc cases add` 命令，这样 LeetCode 有而题目缺少的用例就能加入题目的测试。LeetCode 的结果不会记录到 hc 的历史中。

## 配置

工作区根目录下的 `hc.yaml`：

```yaml
language: go          # 默认解题语言：go | python
editor: nvim          # 未设置时依次使用 $VISUAL、$EDITOR、vi
theme: auto           # 题面配色：auto | dark | light
images: auto          # 题面图片：auto | kitty | blocks | none
timeout: 10s          # 每次运行及其判题程序、生成器的墙钟时限（协议：运行时限）
python:
  interpreter: python3.12  # 代替 .hc/venv 使用的 Python 3.10+，也可以是完整路径；默认不设置
agent:
  name: claude        # claude | codex，或者设置 command（见上文）
leetcode:
  site: com           # com | cn
locales: [en]         # 你自己题目的题面语言，默认的排在第一
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
- 限制运行时间（见[时间限制](#时间限制)），但不限制内存。
- Python 的递归上限就是 Python 自己的，默认 1000：需要更深递归的解答用 `sys.setrecursionlimit` 自行提高，和任何 Python 程序一样。Python 3.11 之前，每次 Python 调用还会占用 C 栈，深递归会撑爆主线程的栈，无论上限设多少，进程都会崩溃。因此在 3.10 上，hc 在一个栈大小为 512 MB 的线程中运行解答、检查器和生成器，足够几十万层调用。
- 无法获取 LeetCode 会员题目。
- 合法答案很多、无法全部列出的题目（"返回任意一个合法答案"）和随机题由[判题程序](protocol_zh.md#判题程序)评测，判题程序由 `hc gen` 编写，回归测试集中的 5、108、210、324、384 和 528 都带有判题程序。判题程序用 Python 编写，所以即使用 Go 作答，这些题目也需要 Python 3.10 或更新版本；hc 管理的虚拟环境会提供它。
