int* preorder(struct Node* root, int* returnSize) {
    int size = 0, capacity = 64, outCapacity = 64;
    struct Node** todo = malloc((size_t)capacity * sizeof(*todo));
    int* out = malloc((size_t)outCapacity * sizeof(int));
    *returnSize = 0;
    if (root) todo[size++] = root;
    while (size) {
        struct Node* n = todo[--size];
        if (*returnSize == outCapacity) {
            outCapacity *= 2;
            out = realloc(out, (size_t)outCapacity * sizeof(int));
        }
        out[(*returnSize)++] = n->val;
        while (size + n->numChildren > capacity) {
            capacity *= 2;
            todo = realloc(todo, (size_t)capacity * sizeof(*todo));
        }
        for (int i = n->numChildren - 1; i >= 0; --i)
            todo[size++] = n->children[i];
    }
    free(todo);
    return out;
}
