struct ListNode** splitListToParts(struct ListNode* head, int k,
                                   int* returnSize) {
    int n = 0;
    for (struct ListNode* p = head; p; p = p->next) ++n;
    struct ListNode** out = calloc((size_t)k, sizeof(*out));
    *returnSize = k;
    for (int i = 0; i < k; ++i) {
        int size = n / k + (i < n % k);
        out[i] = head;
        for (int j = 1; j < size; ++j) head = head->next;
        if (head) {
            struct ListNode* next = head->next;
            head->next = NULL;
            head = next;
        }
    }
    return out;
}
