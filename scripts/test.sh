#!/usr/bin/env bash
# Partition by package, keeping each test in exactly one layer. Packages with
# real compiler/runtime integration live in the integration layer. A new package
# defaults to the base layer so it cannot disappear from the suite.
set -euo pipefail
layer=${1:?expected all, unit, integration or regression}
shift
corpus='^(TestRegressionReferences|TestGoMinimumVersion|TestCPPMinimumVersion|TestCMinimumVersion|TestECMAScriptReferences)$'
case "$layer" in
  all) exec go test -race "$@" ./... ;;
  regression) exec go test -race "$@" -run "$corpus" ./internal/app ;;
  unit|integration) ;;
  *) echo "Unknown test layer: $layer" >&2; exit 1 ;;
esac
packages=()
# TEST_PACKAGES contains trusted Go package patterns, not shell code.
read -r -a patterns <<< "${TEST_PACKAGES:-./...}"
listed=$(go list "${patterns[@]}")
while IFS= read -r package; do
  case "$package" in
    */internal/app|*/internal/cli|*/internal/tui|*/internal/runner|*/internal/leetcode|*/internal/lang|*/internal/lang/*) kind=integration ;;
    *) kind=unit ;;
  esac
  if [[ "$layer" == "$kind" ]]; then packages+=("$package"); fi
done <<< "$listed"
if [[ ${#packages[@]} -eq 0 ]]; then
  echo "No packages selected for $layer" >&2
  exit 1
fi
exec go test -race "$@" -skip "$corpus" "${packages[@]}"
