package main

func splitListToParts(head *ListNode, k int) []*ListNode {
	// Collect the nodes, then cut the list at the start of each part.
	var nodes []*ListNode
	for n := head; n != nil; n = n.Next {
		nodes = append(nodes, n)
	}
	parts := make([]*ListNode, k)
	start := 0
	for i := range parts {
		size := len(nodes) / k
		if i < len(nodes)%k {
			size++
		}
		if size == 0 {
			continue
		}
		parts[i] = nodes[start]
		start += size
		nodes[start-1].Next = nil
	}
	return parts
}
