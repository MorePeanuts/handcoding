package main

func intersect(quadTree1 *Node, quadTree2 *Node) *Node {
	// Paint both trees into one grid (a cell is 1 if either tree has a 1
	// there), then build the quad tree of that grid.
	var depth func(n *Node) int
	depth = func(n *Node) int {
		if n.IsLeaf {
			return 0
		}
		d := 0
		for _, c := range []*Node{n.TopLeft, n.TopRight, n.BottomLeft, n.BottomRight} {
			d = max(d, depth(c))
		}
		return d + 1
	}
	size := 1 << max(depth(quadTree1), depth(quadTree2))
	grid := make([][]bool, size)
	for i := range grid {
		grid[i] = make([]bool, size)
	}
	var paint func(n *Node, r, c, s int)
	paint = func(n *Node, r, c, s int) {
		if n.IsLeaf {
			if n.Val {
				for i := r; i < r+s; i++ {
					for j := c; j < c+s; j++ {
						grid[i][j] = true
					}
				}
			}
			return
		}
		h := s / 2
		paint(n.TopLeft, r, c, h)
		paint(n.TopRight, r, c+h, h)
		paint(n.BottomLeft, r+h, c, h)
		paint(n.BottomRight, r+h, c+h, h)
	}
	paint(quadTree1, 0, 0, size)
	paint(quadTree2, 0, 0, size)

	var build func(r, c, s int) *Node
	build = func(r, c, s int) *Node {
		if s == 1 {
			return &Node{Val: grid[r][c], IsLeaf: true}
		}
		h := s / 2
		tl, tr, bl, br := build(r, c, h), build(r, c+h, h), build(r+h, c, h), build(r+h, c+h, h)
		if tl.IsLeaf && tr.IsLeaf && bl.IsLeaf && br.IsLeaf && tl.Val == tr.Val && tl.Val == bl.Val && tl.Val == br.Val {
			return &Node{Val: tl.Val, IsLeaf: true}
		}
		return &Node{TopLeft: tl, TopRight: tr, BottomLeft: bl, BottomRight: br}
	}
	return build(0, 0, size)
}
