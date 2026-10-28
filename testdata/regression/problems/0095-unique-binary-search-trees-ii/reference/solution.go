package main

func generateTrees(n int) []*TreeNode {
	// Builds every tree from scratch, trying roots from the largest value
	// down, so the trees come out in another order than the Python
	// reference's.
	var build func(lo, hi int) []*TreeNode
	build = func(lo, hi int) []*TreeNode {
		if lo > hi {
			return []*TreeNode{nil}
		}
		var out []*TreeNode
		for root := hi; root >= lo; root-- {
			for _, left := range build(lo, root-1) {
				for _, right := range build(root+1, hi) {
					out = append(out, &TreeNode{Val: root, Left: left, Right: right})
				}
			}
		}
		return out
	}
	return build(1, n)
}
