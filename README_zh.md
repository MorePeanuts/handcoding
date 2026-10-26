<div align="center">

# handcoding

**亲手写代码刷题：用你自己的编辑器，在你自己的机器上判题。**

[![CI](https://github.com/MorePeanuts/handcoding/actions/workflows/ci.yml/badge.svg)](https://github.com/MorePeanuts/handcoding/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/MorePeanuts/handcoding?include_prereleases&sort=semver)](https://github.com/MorePeanuts/handcoding/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/MorePeanuts/handcoding)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) · 简体中文

</div>

<p align="center"><img src="docs/images/demo.gif" alt="用 hc 做 Two Sum：测试失败，在 Neovim 中修复，再提交并查看历史"></p>

> [!WARNING]
> 本项目处于早期开发阶段（`v0.x`）。在 1.0 之前，命令行接口、配置文件、题目格式和数据结构都可能发生不兼容的变化，升级后可能需要重新初始化工作区。

## 为什么做这个工具

编码智能体已经能写出大量上线代码，对资深开发者来说，这是实实在在的效率提升。但要审查、引导和调试这些代码，靠的是判断力，而判断力来自自己亲手写过大量代码。所以练习仍然绕不开：读懂题目，选对数据结构，写错一个下标，再用调试器把它找出来。

在线判题平台让这种练习变得没必要地麻烦：

- 必须联网；
- 调试器要付费；
- 隐藏用例只告诉你错了，不告诉你错在哪；
- 只能在浏览器里写，用不了自己熟悉的编辑器。

命令行客户端解决了编辑器的问题，但代码仍然要发到云端去判。

**handcoding（`hc`）把整个流程搬到本地。** 题目就是磁盘上的文件。你用 Neovim、VS Code 或任何喜欢的编辑器写解答，`hc` 在本机编译、运行、判题和调试，每一个测试用例都是你能直接打开看的文件。

## 功能

- **本地判题**：支持 Go 和 Python，题型包括设计题（`LRUCache`）、原地修改、链表、二叉树、字符矩阵、浮点误差和任意顺序答案。不需要联网。
- **真正的调试器**：`hc debug` 启动 Delve 或 pdb，停在你的函数入口；`--headless` 提供 DAP 服务，可用 nvim-dap 或 VS Code 连接。
- **交互界面**：rustlings 风格的题目列表，题面、配图、测试结果和提交历史都在同一个界面里，编辑器开在旁边的窗格。
- **导入 LeetCode 题目**：`hc fetch 146` 下载题面、配图、函数签名、起始模板和示例。
- **智能体生成测试用例**：`hc gen` 让 Claude Code 或 Codex 判断题目该怎么判（顺序是否重要？是否有多个正确答案？），并写参考解和测试输入。期望输出由 hc 运行参考解得到，而不是让模型猜。用例有错时，用 `hc fix` 修复。
- **开放的题目格式**：一道题就是一个普通目录，由 YAML、Markdown 和 JSON Lines 文件组成。可以自己出题，也可以分享给别人。
- **提交历史**：每次提交都保存代码和判题结果，可以回看、恢复或删除。

## 安装

```sh
go install github.com/MorePeanuts/handcoding/cmd/hc@latest
```

Linux、macOS 和 Windows 的预编译二进制文件见 [Releases 页面](https://github.com/MorePeanuts/handcoding/releases)。

答题需要对应语言的工具链：Go，或 Python 3.8+。以下为可选依赖：

- [Delve](https://github.com/go-delve/delve)：调试 Go；
- `debugpy`：以 headless 模式调试 Python；
- `claude` 或 `codex`：生成测试用例。

## 快速开始

```sh
hc init                 # 创建 ~/handcoding 工作区，附带几道示例题
hc                      # 打开题目列表
```

在列表里选一道题按 `enter`：hc 会显示题面，按模板生成 `solution.go`，并把文件路径复制到剪贴板。在旁边的编辑器里打开它，写完后按 `t` 测试、按 `s` 提交。

> [!TIP]
> 题面图片在 kitty、Ghostty 中以原图显示。在 [herdr](https://herdr.dev) 中，只要 herdr 外层的终端支持 kitty 图片协议，也会显示原图。如果在 herdr 中图片区域显示为空白，说明外层终端不支持 kitty 图片协议，请在 `hc.yaml` 中设置 `images: blocks`。详见[使用指南](docs/how-to-use_zh.md#配置)。

界面里的每个操作也都有对应的命令：

```sh
hc fetch 206            # 从 LeetCode 导入一道题
hc gen 206              # 让编码智能体生成测试用例和判题规则
hc test two-sum         # 运行示例用例
hc submit two-sum       # 用全部用例判题，并记录这次提交
hc debug two-sum -c 4   # 调试第 4 个用例
```

## 文档

文档目前只有英文版：

- [使用指南](docs/how-to-use_zh.md)：交互界面、全部命令、配置、调试和智能体
- [题目协议](docs/protocol_zh.md)：如何编写自己的题目，以及如何扩展新语言

## 路线图

接下来计划做的：

- 更多语言：C、C++、Rust、Java、JavaScript、TypeScript；
- 题目包，以及官方维护的 LeetCode 题库；
- 题目笔记，以及只给提示、绝不直接给答案的 AI 辅导；
- 开放的题目商店。

进度见 [Milestones](https://github.com/MorePeanuts/handcoding/milestones)。

## 参与贡献

欢迎在 [Issues](https://github.com/MorePeanuts/handcoding/issues) 中报告问题或提出建议。构建和测试方法见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

[MIT](LICENSE)
