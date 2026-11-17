from hc_runtime import MultilevelNode as Node


class Solution:
    def flatten(self, head: Node | None) -> Node | None:
        # Splice each child list in place, then continue with its first node.
        node = head
        while node:
            if node.child:
                child, tail = node.child, node.child
                while tail.next:
                    tail = tail.next
                tail.next = node.next
                if node.next:
                    node.next.prev = tail
                node.next, child.prev, node.child = child, node, None
            node = node.next
        return head
