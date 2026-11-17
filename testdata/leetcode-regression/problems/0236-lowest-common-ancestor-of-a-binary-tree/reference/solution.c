typedef struct Parent {
    struct TreeNode *node, *parent;
    bool ancestor;
    UT_hash_handle hh;
} Parent;
struct TreeNode* lowestCommonAncestor(struct TreeNode* root, struct TreeNode* p,
                                      struct TreeNode* q) {
    Parent *parents = NULL, *entry, *tmp;
    int size = 0, capacity = 64;
    struct TreeNode** todo = malloc((size_t)capacity * sizeof(*todo));
    entry = calloc(1, sizeof(*entry));
    entry->node = root;
    HASH_ADD_PTR(parents, node, entry);
    todo[size++] = root;
    while (size) {
        struct TreeNode* n = todo[--size];
        struct TreeNode* children[] = {n->left, n->right};
        for (int i = 0; i < 2; ++i)
            if (children[i]) {
                entry = calloc(1, sizeof(*entry));
                entry->node = children[i];
                entry->parent = n;
                HASH_ADD_PTR(parents, node, entry);
                if (size == capacity) {
                    capacity *= 2;
                    todo = realloc(todo, (size_t)capacity * sizeof(*todo));
                }
                todo[size++] = children[i];
            }
    }
    for (struct TreeNode* n = p; n;) {
        HASH_FIND_PTR(parents, &n, entry);
        entry->ancestor = true;
        n = entry->parent;
    }
    while (q) {
        HASH_FIND_PTR(parents, &q, entry);
        if (entry->ancestor) break;
        q = entry->parent;
    }
    HASH_ITER(hh, parents, entry, tmp) {
        HASH_DEL(parents, entry);
        free(entry);
    }
    free(todo);
    return q;
}
