package main

func mergeKLists(lists []*ListNode) *ListNode {
	// Divide and conquer: merge pairs of lists until one is left.
	merge := func(a, b *ListNode) *ListNode {
		dummy := &ListNode{}
		tail := dummy
		for a != nil && b != nil {
			if a.Val <= b.Val {
				tail.Next, a = a, a.Next
			} else {
				tail.Next, b = b, b.Next
			}
			tail = tail.Next
		}
		if a != nil {
			tail.Next = a
		} else {
			tail.Next = b
		}
		return dummy.Next
	}
	if len(lists) == 0 {
		return nil
	}
	for len(lists) > 1 {
		var next []*ListNode
		for i := 0; i < len(lists); i += 2 {
			if i+1 < len(lists) {
				next = append(next, merge(lists[i], lists[i+1]))
			} else {
				next = append(next, lists[i])
			}
		}
		lists = next
	}
	return lists[0]
}
