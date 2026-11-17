# Regression packs

| Pack | Purpose |
|---|---|
| `leetcode-regression` | 49 LeetCode problems covering native interfaces and judging behavior |
| `protocol-regression` | hc protocol extensions; currently `prefix-sums-3d` for three-dimensional array inputs and outputs |

Both packs provide reference solutions in all seven supported languages.
LeetCode templates are included where available; some problems lack templates
in languages that the public API does not provide.

## Run tests

```sh
make test-regression
HC_TEST_LANGUAGES=c HC_TEST_PROBLEMS=two-sum make test-regression
HC_TEST_PROBLEMS=prefix-sums-3d make test-regression
```

The corpus tests build hc once per test process, install both packs into
temporary workspaces with `hc pack install`, then run their references.
Protocol coverage and reference checks inspect both packs.

## Use a pack manually

From the repository root:

```sh
hc init /tmp/hc-dev
hc -w /tmp/hc-dev pack install testdata/leetcode-regression
hc -w /tmp/hc-dev pack install testdata/protocol-regression
hc -w /tmp/hc-dev start leetcode-regression/two-sum -l go
# Edit the solution file printed by start, then run it:
hc -w /tmp/hc-dev test leetcode-regression/two-sum -l go
```

Each pack is also an authoring workspace. To validate its reference solutions:

```sh
hc -w testdata/leetcode-regression validate --run
hc -w testdata/protocol-regression validate --run
```
