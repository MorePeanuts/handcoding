class Solution {
   public:
    ListNode* getIntersectionNode(ListNode* a, ListNode* b) {
        auto x = a, y = b;
        while (x != y) {
            x = x ? x->next : b;
            y = y ? y->next : a;
        }
        return x;
    }
};
