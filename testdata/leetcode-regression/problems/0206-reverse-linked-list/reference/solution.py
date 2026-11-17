from hc_runtime import ListNode


class Solution:
    def reverseList(self, head: ListNode | None) -> ListNode | None:
        prev = None
        while head:
            head.next, prev, head = prev, head, head.next
        return prev
