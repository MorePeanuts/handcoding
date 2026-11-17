package main

import "slices"

func groupAnagrams(strs []string) [][]string {
	groups := map[string][]string{}
	for _, s := range strs {
		b := []byte(s)
		slices.Sort(b)
		groups[string(b)] = append(groups[string(b)], s)
	}
	res := make([][]string, 0, len(groups))
	for _, g := range groups {
		res = append(res, g)
	}
	return res
}
