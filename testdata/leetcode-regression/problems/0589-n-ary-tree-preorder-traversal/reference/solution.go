package main

func preorder(root *Node) []int {
	out := []int{}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		out = append(out, n.Val)
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return out
}
