struct Node* copyRandomList(struct Node* head) {
    for (struct Node* n = head; n;) {
        struct Node* copy = calloc(1, sizeof(*copy));
        copy->val = n->val;
        copy->next = n->next;
        n->next = copy;
        n = copy->next;
    }
    for (struct Node* n = head; n; n = n->next->next)
        n->next->random = n->random ? n->random->next : NULL;
    struct Node* out = head ? head->next : NULL;
    for (struct Node* n = head; n;) {
        struct Node *copy = n->next, *next = copy->next;
        n->next = next;
        copy->next = next ? next->next : NULL;
        n = next;
    }
    return out;
}
