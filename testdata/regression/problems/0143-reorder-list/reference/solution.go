package main

func reorderList(head *ListNode) {
	// Split at the middle, reverse the second half, then merge.
	slow, fast := head, head
	for fast.Next != nil && fast.Next.Next != nil {
		slow, fast = slow.Next, fast.Next.Next
	}
	var rev *ListNode
	for cur := slow.Next; cur != nil; {
		cur.Next, rev, cur = rev, cur, cur.Next
	}
	slow.Next = nil
	for a, b := head, rev; b != nil; {
		an, bn := a.Next, b.Next
		a.Next, b.Next = b, an
		a, b = an, bn
	}
}
