# 回归测试题目包

| 题目包 | 用途 |
|---|---|
| `leetcode-regression` | 49 道 LeetCode 题目，验证原生接口和判题行为 |
| `protocol-regression` | hc 协议扩展；目前通过 `prefix-sums-3d` 验证三维数组输入和输出 |

两个包都提供全部七种支持语言的参考答案。LeetCode 模板按可用情况提供；
公开 API 未提供对应语言模板的题目可能缺少该语言的模板。

## 运行测试

```sh
make test-regression
HC_TEST_LANGUAGES=c HC_TEST_PROBLEMS=two-sum make test-regression
HC_TEST_PROBLEMS=prefix-sums-3d make test-regression
```

题库回归测试每个测试进程只构建一次 hc，通过 `hc pack install` 将两个包
安装到临时工作区后运行参考答案。协议覆盖和参考答案检查同时检查两个包。

## 手动使用题目包

在仓库根目录执行：

```sh
hc init /tmp/hc-dev
hc -w /tmp/hc-dev pack install testdata/leetcode-regression
hc -w /tmp/hc-dev pack install testdata/protocol-regression
hc -w /tmp/hc-dev start leetcode-regression/two-sum -l go
# 编辑 start 输出的解答文件路径，然后运行：
hc -w /tmp/hc-dev test leetcode-regression/two-sum -l go
```

每个包也是一个题目包开发工作区。验证其中的参考答案：

```sh
hc -w testdata/leetcode-regression validate --run
hc -w testdata/protocol-regression validate --run
```
