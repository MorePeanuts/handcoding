class Solution {
   public:
    void deleteNode(ListNode* node) {
        auto removed = node->next;
        node->val = removed->val;
        node->next = removed->next;
        delete removed;
    }
};
