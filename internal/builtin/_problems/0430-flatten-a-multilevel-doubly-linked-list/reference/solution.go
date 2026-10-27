package main

func flatten(head *Node) *Node {
	// flat flattens the list starting at n and returns its last node.
	var flat func(n *Node) *Node
	flat = func(n *Node) *Node {
		var last *Node
		for n != nil {
			next := n.Next
			if n.Child != nil {
				childLast := flat(n.Child)
				n.Next, n.Child.Prev = n.Child, n
				n.Child = nil
				childLast.Next = next
				if next != nil {
					next.Prev = childLast
				}
				last = childLast
			} else {
				last = n
			}
			n = next
		}
		return last
	}
	flat(head)
	return head
}
