package main

func getIntersectionNode(headA, headB *ListNode) *ListNode {
	// Both pointers walk A then B (or B then A), so they meet at the
	// intersection, or at nil after m+n steps.
	a, b := headA, headB
	for a != b {
		if a == nil {
			a = headB
		} else {
			a = a.Next
		}
		if b == nil {
			b = headA
		} else {
			b = b.Next
		}
	}
	return a
}
