typedef struct {
    struct TreeNode *original, *clone;
} Pair;
struct TreeNode* getTargetCopy(struct TreeNode* original,
                               struct TreeNode* cloned,
                               struct TreeNode* target) {
    int size = 0, capacity = 64;
    Pair* todo = malloc((size_t)capacity * sizeof(*todo));
    todo[size++] = (Pair){original, cloned};
    struct TreeNode* result = NULL;
    while (size) {
        Pair p = todo[--size];
        if (p.original == target) {
            result = p.clone;
            break;
        }
        if (size + 2 > capacity) {
            capacity *= 2;
            todo = realloc(todo, (size_t)capacity * sizeof(*todo));
        }
        if (p.original->left)
            todo[size++] = (Pair){p.original->left, p.clone->left};
        if (p.original->right)
            todo[size++] = (Pair){p.original->right, p.clone->right};
    }
    free(todo);
    return result;
}
