typedef struct {
    struct Node *source, *copy;
} Pair;
struct Node* cloneTree(struct Node* root) {
    if (!root) return NULL;
    struct Node* out = calloc(1, sizeof(*out));
    int size = 0, capacity = 64;
    Pair* todo = malloc((size_t)capacity * sizeof(*todo));
    todo[size++] = (Pair){root, out};
    while (size) {
        Pair p = todo[--size];
        p.copy->val = p.source->val;
        p.copy->numChildren = p.source->numChildren;
        p.copy->children =
            malloc((size_t)p.copy->numChildren * sizeof(*p.copy->children));
        while (size + p.copy->numChildren > capacity) {
            capacity *= 2;
            todo = realloc(todo, (size_t)capacity * sizeof(*todo));
        }
        for (int i = 0; i < p.copy->numChildren; ++i) {
            p.copy->children[i] = calloc(1, sizeof(struct Node));
            todo[size++] = (Pair){p.source->children[i], p.copy->children[i]};
        }
    }
    free(todo);
    return out;
}
