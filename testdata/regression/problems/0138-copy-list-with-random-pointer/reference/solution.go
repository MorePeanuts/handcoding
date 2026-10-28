package main

func copyRandomList(head *Node) *Node {
	// Weave a copy after each node, set the copies' random pointers, then
	// unweave the two lists.
	for n := head; n != nil; n = n.Next.Next {
		n.Next = &Node{Val: n.Val, Next: n.Next}
	}
	for n := head; n != nil; n = n.Next.Next {
		if n.Random != nil {
			n.Next.Random = n.Random.Next
		}
	}
	dummy := &Node{}
	tail := dummy
	for n := head; n != nil; n = n.Next {
		tail.Next = n.Next
		tail = tail.Next
		n.Next = tail.Next
	}
	return dummy.Next
}
