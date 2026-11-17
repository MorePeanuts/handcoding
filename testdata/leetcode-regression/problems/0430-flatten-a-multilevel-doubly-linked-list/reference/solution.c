struct Node* flatten(struct Node* head) {
    if (!head) return NULL;
    int size = 0, capacity = 16;
    struct Node** todo = malloc((size_t)capacity * sizeof(*todo));
    todo[size++] = head;
    struct Node* prev = NULL;
    while (size) {
        struct Node* n = todo[--size];
        if (size + 2 > capacity) {
            capacity *= 2;
            todo = realloc(todo, (size_t)capacity * sizeof(*todo));
        }
        if (n->next) todo[size++] = n->next;
        if (n->child) todo[size++] = n->child;
        n->child = NULL;
        n->prev = prev;
        if (prev) prev->next = n;
        prev = n;
    }
    prev->next = NULL;
    free(todo);
    return head;
}
