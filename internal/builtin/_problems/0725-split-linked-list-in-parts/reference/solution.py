from hc_runtime import ListNode


class Solution:
    def splitListToParts(self, head: ListNode | None, k: int) -> list[ListNode | None]:
        n, node = 0, head
        while node:
            n, node = n + 1, node.next
        size, extra = divmod(n, k)
        parts: list[ListNode | None] = []
        node = head
        for i in range(k):
            parts.append(node)
            for _ in range(size + (i < extra) - 1):
                node = node.next
            if node:
                node.next, node = None, node.next
        return parts
