from hc_runtime import ListNode


class Solution:
    def detectCycle(self, head: ListNode | None) -> ListNode | None:
        seen = set()
        while head:
            if id(head) in seen:
                return head
            seen.add(id(head))
            head = head.next
        return None
