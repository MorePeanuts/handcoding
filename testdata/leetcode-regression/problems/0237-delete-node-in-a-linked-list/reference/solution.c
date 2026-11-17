void deleteNode(struct ListNode* node) {
    struct ListNode* removed = node->next;
    node->val = removed->val;
    node->next = removed->next;
    free(removed);
}
