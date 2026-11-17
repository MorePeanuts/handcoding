#!/usr/bin/env bash
# Partition by package, keeping each test in exactly one layer. Packages with
# real compiler/runtime integration live in the integration layer. A new package
# defaults to the base layer so it cannot disappear from the suite.
set -euo pipefail
layer=${1:?expected all, unit, integration or regression}
shift
phase_event() {
  printf '{"HCPhase":"%s","Layer":"%s","Time":"%s","Status":%s}\n' "$1" "$layer" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${2:-0}"
}
phase_event start
trap 'phase_event end "$?"' EXIT
corpus='^(TestRegressionReferences|TestGoMinimumVersion|TestCPPMinimumVersion|TestCMinimumVersion|TestECMAScriptReferences)$'
case "$layer" in
  all) go test -race "$@" ./...; exit ;;
  regression) go test -race "$@" -run "$corpus" ./internal/app; exit ;;
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
# CI can overlap packages after app's timing-sensitive tests finish. Bound
# both package and per-package concurrency so native compilation cannot fan
# out as packages * four tests. Local invocations retain their own Go flags.
if [[ "$layer" == integration && -n "${TEST_PACKAGE_JOBS:-}" ]]; then
  if [[ ! "$TEST_PACKAGE_JOBS" =~ ^[1-9][0-9]*$ || ! "${TEST_PACKAGE_PARALLEL:-2}" =~ ^[1-9][0-9]*$ ]]; then
    echo "TEST_PACKAGE_JOBS and TEST_PACKAGE_PARALLEL must be positive integers" >&2
    exit 1
  fi
  concurrent=()
  for package in "${packages[@]}"; do
    if [[ "$package" == */internal/app ]]; then
      go test -race "$@" -p=1 -skip "$corpus" "$package"
    else
      concurrent+=("$package")
    fi
  done
  if [[ ${#concurrent[@]} -gt 0 ]]; then
    go test -race "$@" -p="$TEST_PACKAGE_JOBS" -parallel="${TEST_PACKAGE_PARALLEL:-2}" -skip "$corpus" "${concurrent[@]}"
  fi
else
  go test -race "$@" -skip "$corpus" "${packages[@]}"
fi
