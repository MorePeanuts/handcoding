struct ListNode* getIntersectionNode(struct ListNode* a, struct ListNode* b) {
    struct ListNode *x = a, *y = b;
    while (x != y) {
        x = x ? x->next : b;
        y = y ? y->next : a;
    }
    return x;
}
