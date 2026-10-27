package main

func cloneGraph(node *Node) *Node {
	clones := map[*Node]*Node{}
	var clone func(n *Node) *Node
	clone = func(n *Node) *Node {
		if n == nil {
			return nil
		}
		if c, ok := clones[n]; ok {
			return c
		}
		c := &Node{Val: n.Val}
		clones[n] = c
		for _, nb := range n.Neighbors {
			c.Neighbors = append(c.Neighbors, clone(nb))
		}
		return c
	}
	return clone(node)
}
