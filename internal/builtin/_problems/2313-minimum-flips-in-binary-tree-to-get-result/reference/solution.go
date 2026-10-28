package main

func minimumFlips(root *TreeNode, result bool) int {
	// flips(n, want) tries every combination of child values that gives
	// want, memoized per node and wanted value.
	type key struct {
		n    *TreeNode
		want bool
	}
	memo := map[key]int{}
	var flips func(n *TreeNode, want bool) int
	flips = func(n *TreeNode, want bool) int {
		if n.Val <= 1 {
			if (n.Val == 1) == want {
				return 0
			}
			return 1
		}
		k := key{n, want}
		if v, ok := memo[k]; ok {
			return v
		}
		var best int
		if n.Val == 5 {
			child := n.Left
			if child == nil {
				child = n.Right
			}
			best = flips(child, !want)
		} else {
			best = -1
			for _, a := range []bool{false, true} {
				for _, b := range []bool{false, true} {
					var got bool
					switch n.Val {
					case 2:
						got = a || b
					case 3:
						got = a && b
					case 4:
						got = a != b
					}
					if got != want {
						continue
					}
					if c := flips(n.Left, a) + flips(n.Right, b); best < 0 || c < best {
						best = c
					}
				}
			}
		}
		memo[k] = best
		return best
	}
	return flips(root, result)
}
