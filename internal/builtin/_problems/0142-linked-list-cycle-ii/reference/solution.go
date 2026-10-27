package main

func detectCycle(head *ListNode) *ListNode {
	// Floyd: after the pointers meet, the entry is as far from the head as
	// from the meeting point.
	slow, fast := head, head
	for fast != nil && fast.Next != nil {
		slow, fast = slow.Next, fast.Next.Next
		if slow == fast {
			for p := head; ; p, slow = p.Next, slow.Next {
				if p == slow {
					return p
				}
			}
		}
	}
	return nil
}
