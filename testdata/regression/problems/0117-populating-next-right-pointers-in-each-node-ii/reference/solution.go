package main

func connect(root *Node) *Node {
	// Walk each level through the next pointers already set, linking the
	// level below; constant extra space.
	for head := root; head != nil; {
		dummy := &Node{}
		tail := dummy
		for n := head; n != nil; n = n.Next {
			for _, c := range []*Node{n.Left, n.Right} {
				if c != nil {
					tail.Next = c
					tail = c
				}
			}
		}
		head = dummy.Next
	}
	return root
}
