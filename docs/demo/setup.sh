#!/bin/sh
# Prepares a throwaway workspace for demo.tape and prints the environment to
# eval: hc built from this checkout, and a Two Sum solution with a bug.
# HOME points at a temporary directory so paths show up as ~/handcoding/...
set -e
root=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
go build -C "$root" -o "$tmp/bin/hc" ./cmd/hc
export GOCACHE="$(go env GOCACHE)" GOPATH="$(go env GOPATH)" # keep builds warm
export PATH="$tmp/bin:$PATH" HOME="$tmp" HC_WORKSPACE="$tmp/handcoding"
printf 'set tabstop=4 shiftwidth=4\n' >"$tmp/init.vim"

hc init "$HC_WORKSPACE" >/dev/null
hc fetch two-sum --force >/dev/null # English statement from leetcode.com
cfg="$HC_WORKSPACE/hc.yaml"
sed -i.bak 's/^theme: auto/theme: dark/' "$cfg" && rm "$cfg.bak"
echo "editor: nvim --clean -u $tmp/init.vim" >>"$cfg"
hc start two-sum >/dev/null 2>&1
cat >"$HC_WORKSPACE/solutions/go/0001-two-sum/solution.go" <<'GO'
package main

func twoSum(nums []int, target int) []int {
	for i := range nums {
		for j := i; j < len(nums); j++ {
			if nums[i]+nums[j] == target {
				return []int{i, j}
			}
		}
	}
	return nil
}
GO

echo "export PATH='$PATH' HOME='$HOME' GOCACHE='$GOCACHE' GOPATH='$GOPATH' HC_WORKSPACE='$HC_WORKSPACE' PS1='\$ '"
