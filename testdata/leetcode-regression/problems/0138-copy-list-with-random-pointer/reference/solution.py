from hc_runtime import RandomListNode as Node


class Solution:
    def copyRandomList(self, head: Node | None) -> Node | None:
        clones = {None: None}
        node = head
        while node:
            clones[node] = Node(node.val)
            node = node.next
        node = head
        while node:
            clones[node].next = clones[node.next]
            clones[node].random = clones[node.random]
            node = node.next
        return clones[head]
