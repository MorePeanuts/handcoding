from hc_runtime import ListNode


class Solution:
    def deleteNode(self, node: ListNode | None) -> None:
        node.val = node.next.val
        node.next = node.next.next
