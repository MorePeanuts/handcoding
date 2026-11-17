void reorderList(struct ListNode* head) {
    if (!head || !head->next) return;
    struct ListNode *slow = head, *fast = head;
    while (fast->next && fast->next->next) {
        slow = slow->next;
        fast = fast->next->next;
    }
    struct ListNode *p = slow->next, *rev = NULL;
    slow->next = NULL;
    while (p) {
        struct ListNode* next = p->next;
        p->next = rev;
        rev = p;
        p = next;
    }
    while (rev) {
        struct ListNode *a = head->next, *b = rev->next;
        head->next = rev;
        rev->next = a;
        head = a;
        rev = b;
    }
}
