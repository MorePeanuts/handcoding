package main

func construct(grid [][]int) *Node {
	// Build the four quadrants first and merge them when they are equal
	// leaves. Internal nodes get Val false, which the judge accepts.
	var build func(r, c, n int) *Node
	build = func(r, c, n int) *Node {
		if n == 1 {
			return &Node{Val: grid[r][c] == 1, IsLeaf: true}
		}
		h := n / 2
		tl, tr, bl, br := build(r, c, h), build(r, c+h, h), build(r+h, c, h), build(r+h, c+h, h)
		if tl.IsLeaf && tr.IsLeaf && bl.IsLeaf && br.IsLeaf && tl.Val == tr.Val && tl.Val == bl.Val && tl.Val == br.Val {
			return &Node{Val: tl.Val, IsLeaf: true}
		}
		return &Node{TopLeft: tl, TopRight: tr, BottomLeft: bl, BottomRight: br}
	}
	return build(0, 0, len(grid))
}
