from hc_runtime import ListNode


class Solution:
    def getIntersectionNode(self, headA: ListNode, headB: ListNode) -> ListNode | None:
        seen = set()
        while headA:
            seen.add(id(headA))
            headA = headA.next
        while headB:
            if id(headB) in seen:
                return headB
            headB = headB.next
        return None
