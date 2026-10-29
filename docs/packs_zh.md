# 编写题目包

[English](packs.md) · 简体中文

> 本文是 [docs/packs.md](packs.md) 的译文，两者不一致时以英文为准。

[题目包](protocol_zh.md#题目包)是一起发布的一组题目，通常是一个独立的 git 仓库，任何人都可以用 `hc pack install` 把它安装到自己的工作区。本指南讲如何编写题目包：创建仓库、添加题目、在 CI 中检查、发布版本。题目包及其题目的格式由[题目协议](protocol_zh.md)规定；本指南只链接到协议，不重复其内容。

- [创建题目包](#创建题目包)
- [添加题目](#添加题目)
- [CI 与固定的 hc 版本](#ci-与固定的-hc-版本)
- [发布](#发布)
- [信任](#信任)

## 创建题目包

```sh
hc pack init my-pack --title "My pack"   # 或者在一个空目录中：hc pack init
cd my-pack
git init                                  # 如果它还不是仓库
```

`hc pack init [dir]` 在 `dir`（默认为当前目录）中生成一个题目包的骨架，目录不存在时会先创建：

```
pack.yaml                     清单：protocol、name、title 和一个空的 groups 列表
problems/                     题目，每题一个目录（初始为空，带一个 .gitkeep）
.gitignore                    hc 在检出目录中添加的内容：.hc/、solutions/、hc.yaml、pyrightconfig.json
.github/workflows/check.yml   CI：每次 push 和 pull request 时运行 hc pack check
AGENTS.md                     给编码智能体的说明，以及 hc 的技能索引
CLAUDE.md                     @AGENTS.md，以及留给 Claude Code 的备注
.claude/skills/               编写题目包用的技能，供 Claude Code 使用
.agents/skills/               同样的技能，供 Codex 使用
```

把这些全部提交。技能是有意提交的：新克隆的仓库可以向任何打开它的智能体说明自己，CI 也会检查这些技能属于它所固定的 hc（见[下文](#ci-与固定的-hc-版本)）。给智能体的备注写在 `AGENTS.md` 中 `<!-- hc:begin -->` 与 `<!-- hc:end -->` 之间那一段之外，这一段由 hc 重写；`CLAUDE.md` 完全属于你。

- **名称**取自 `--name`，否则取目录名的 kebab-case 形式（`My Pack` 变成 `my-pack`）。名称是题目包的标识：已安装的题目包、解答和历史记录都以它为键，所以要在发布前定好。名称规则见 [`pack.yaml`](protocol_zh.md#packyaml)。
- **拒绝的情况。** `hc pack init` 拒绝已经包含 `pack.yaml` 的目录和位于某个工作区内的目录，并且从不覆盖文件：只要它要写的任何一个文件已经存在，它就把这些文件列出来，什么也不写。
- **git。** hc 不会运行 `git init`。如果 `dir` 已经在某个 git 仓库中，它仍然属于该仓库；但 GitHub 只运行仓库根目录下的工作流，所以题目包应当就是仓库的根目录。

此后这个目录就是题目包的检出目录，hc 直接把它作为工作区打开（见[编写题目包](how-to-use_zh.md#编写题目包)）：在其中运行的每个 hc 命令都作用于这个题目包。

`pack.yaml` 能写的不止 `hc pack init` 写入的这些：`version`、`description`、`authors`、`license`、`homepage`、题目可以用哪些语言解答的 `languages`、题面使用哪些语言的 `locales`，以及把题目编排成有序列表（如专题或学习计划）的 `groups`。`hc docs protocol packyaml` 打印这些字段，`hc docs protocol groups` 打印分组的规则。

**多语言题面。** 题目包可以用多种语言提供题面：`locales: [en, zh-CN]`。用哪些语言、哪个排在第一，由你决定：第一个是默认语言区域，也就是每道题 `description.md` 和 `title` 的语言。其他每个语言区域，每道题都需要一个 `description.<locale>.md`，以及 `problem.yaml` 中的 `titles.<locale>`；题目包和分组的 `title`、`titles` 是可选的。`hc docs protocol locales` 列出这些规则。`hc fetch` 按题目包的语言区域写入 LeetCode 提供的题面（两个站点都有英文，leetcode.cn 还有中文），`hc validate` 和 `hc pack check` 会报告缺少的或未声明的语言区域。hc 中没有任何东西会翻译题面，遵循 hc 的 skill 的智能体也不会：LeetCode 没有的，要你自己写。

## 添加题目

```sh
hc fetch two-sum            # 从 LeetCode 导入一道题
hc new my-problem           # 或者自己出一道题（problems/my-problem/）
hc import ../elsewhere/     # 或者把题目目录复制进来
hc gen two-sum              # 让智能体编写参考解、用例和判题规则
hc validate two-sum --run   # 每个参考解都通过每个用例
hc pack check               # 像 CI 那样检查整个题目包
```

流程与练习工作区相同（见[自己出题](how-to-use_zh.md#自己出题)、[用编码智能体生成用例](how-to-use_zh.md#用编码智能体生成用例)），只是题目写到题目包的 `problems/` 下，并且只用 slug 来指代。如果题目包有分组，再把 slug 加到 `pack.yaml` 的某个分组中。

只有通过 `hc pack check` 的题目才能合入：它合法、[已就绪](protocol_zh.md#就绪)（判题规则 `judge.compare` 已经确定）、每个用例都有期望输出，并且每个参考解都通过每个用例。参考解必须能在协议支持的最旧工具链上运行（见[语言版本](protocol_zh.md#语言版本)），CI 用 Python 3.10 运行 Python 参考解。`hc fetch` 有意不设置判题规则，因为确定它需要阅读题面：这是 `hc gen` 的工作。

如果某道题无法用协议表达，不要绕过去（比如用一个判别别的东西的判题程序，或者改动题面去迁就）：去提一个 [issue](https://github.com/MorePeanuts/handcoding/issues)，说明缺少什么，并在协议支持之前先不收录这道题。`hc pack init` 写入的 `AGENTS.md` 也这样告诉智能体。

## CI 与固定的 hc 版本

`.github/workflows/check.yml` 在每次 push 和 pull request 时运行。它安装 Go 和 Python 3.10（协议支持的最旧 Python），从而在用户可能使用的环境中检查参考解和判题程序，然后：

1. 用 `go install github.com/MorePeanuts/handcoding/cmd/hc@<version>` 安装 hc；
2. 写入一个设置了 `python.interpreter: python3` 的 `hc.yaml`，让 hc 用这个 Python 判题，而不是创建虚拟环境（`hc.yaml` 不提交）；
3. 运行 `hc pack check`；
4. 运行 `hc skill install --force`，如果它改动了任何已提交的技能或 `AGENTS.md` 就失败：仓库中的技能必须属于检查这个题目包的 hc。

**固定版本。** `<version>` 是运行 `hc pack init` 的 hc 的版本。hc 的开发版没有可以固定的发布版本，这时工作流安装 `@latest`，并附一条注释提醒你固定它：把 `latest` 换成一个发布版本，例如 `v0.2.0`。固定 hc 的版本可以避免 hc 发布新版本时 CI 跟着改变。

**何时升级。** 固定的 hc 决定 CI 能读取哪些[协议版本](protocol_zh.md#协议版本)，以及检查得多严格。当你需要新版 hc 带来的东西时再升级：新协议的某个特性，或者对检查的修复。升级之前，先用新版 hc 的 `hc docs protocol protocol-versions` 和发布说明了解变化。升级 hc 不会改变 `pack.yaml` 的 `protocol`。只有在要使用某个协议版本引入的特性时才提高它，并且要知道，不支持新协议的 hc 版本会因此拒绝这个题目包。

**刷新技能。** 技能属于某个 hc 版本。升级之后，在本地安装同一版本的 hc，然后：

```sh
hc skill install     # 换成这个 hc 的技能
git diff             # 检查改动，然后和升级一起提交
```

不这样做，CI 的最后一步就会失败。CI 运行的是 `hc skill install --force`，所以仓库中 hc 技能的副本必须与固定的 hc 完全一致：编辑过其中任何一个（`hc skill list` 显示为 *modified*），CI 同样会失败。你自己的说明请写在 `CLAUDE.md`、`AGENTS.md` 的作者部分，或你自己的技能中，hc 从不触碰它们。

## 发布

把仓库推送到某个 git 托管服务。用户用它的 URL 安装：

```sh
hc pack install https://github.com/you/my-pack.git               # 默认分支
hc pack install https://github.com/you/my-pack.git --ref v0.3.0  # 某个发布版本
```

- **发布版本**就是 git 标签。给一个 CI 通过的提交打上 `v0.3.0` 这样的标签，并在这个提交中把 `pack.yaml` 的 `version` 设为 `0.3.0`（SemVer，不带 `v`；见 [`pack.yaml`](protocol_zh.md#packyaml)）。`version` 只是说明性的：hc 按 commit 固定安装，并在 `hc pack list` 中显示这个版本。
- **更新。** `hc pack update` 会把跟随分支的安装移到该分支的最新提交；跟随标签的安装保持不动，除非标签被移动过：用户用 `hc pack update <name> --ref v0.4.0` 切换发布版本。所以标签一经发布就不要再移动。
- **破坏性变更。** 用户的解答和历史记录以题目包的名称、每道题的 slug 和目录为键（见[标识](protocol_zh.md#标识)）。改名的题目包就成了另一个题目包，hc 会拒绝跨改名的更新。改名或删除一道题，会让用户为它做的工作成为孤儿：hc 保留它们并提示，但不再有题目使用它们。题目一经发布，就保持 slug 和目录名不变。

## 信任

题目包就是代码。安装一个题目包，意味着在你的机器上运行它的判题程序和参考解：用判题程序判题的题目每次测试或提交时都会运行判题程序，`hc cases build` 和 `hc validate --run` 会运行参考解，`hc pack check` 两者都运行。hc 不会把它们放进沙箱。因此：

- 作为作者，让参考解和判题程序只做计算答案的事：不访问网络，不碰输入之外的文件，不做读者不会预期一份解答会做的事；
- 作为用户，只从你信任的来源安装题目包，并在安装或更新之前审阅它（它的 `reference/` 和 `checker.py` 文件）；
- 在 CI 中，工作流对仓库只有只读权限（`permissions: contents: read`），所以修改参考解的 pull request 无法向仓库推送。
