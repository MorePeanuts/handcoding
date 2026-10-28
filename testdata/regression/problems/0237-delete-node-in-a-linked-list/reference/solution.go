package main

func deleteNode(node *ListNode) {
	// Shift every later value one node forward, then drop the tail.
	for node.Next.Next != nil {
		node.Val = node.Next.Val
		node = node.Next
	}
	node.Val = node.Next.Val
	node.Next = nil
}
