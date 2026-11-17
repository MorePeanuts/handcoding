static struct ListNode* merge(struct ListNode* a, struct ListNode* b) {
    struct ListNode dummy = {0}, *tail = &dummy;
    while (a && b) {
        if (a->val <= b->val) {
            tail->next = a;
            a = a->next;
        } else {
            tail->next = b;
            b = b->next;
        }
        tail = tail->next;
    }
    tail->next = a ? a : b;
    return dummy.next;
}
struct ListNode* mergeKLists(struct ListNode** lists, int listsSize) {
    for (int step = 1; step < listsSize; step *= 2)
        for (int i = 0; i + step < listsSize; i += 2 * step)
            lists[i] = merge(lists[i], lists[i + step]);
    return listsSize ? lists[0] : NULL;
}
