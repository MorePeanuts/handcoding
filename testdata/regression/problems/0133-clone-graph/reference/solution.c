typedef struct Copy {
    struct Node *source, *target;
    UT_hash_handle hh;
} Copy;
static struct Node* clone(struct Node* n, Copy** copies) {
    if (!n) return NULL;
    Copy* entry;
    HASH_FIND_PTR(*copies, &n, entry);
    if (entry) return entry->target;
    entry = malloc(sizeof(*entry));
    entry->source = n;
    struct Node* out = calloc(1, sizeof(*out));
    entry->target = out;
    HASH_ADD_PTR(*copies, source, entry);
    out->val = n->val;
    out->numNeighbors = n->numNeighbors;
    out->neighbors = malloc((size_t)n->numNeighbors * sizeof(*out->neighbors));
    for (int i = 0; i < n->numNeighbors; ++i)
        out->neighbors[i] = clone(n->neighbors[i], copies);
    return out;
}
struct Node* cloneGraph(struct Node* node) {
    Copy *copies = NULL, *entry, *tmp;
    struct Node* out = clone(node, &copies);
    HASH_ITER(hh, copies, entry, tmp) {
        HASH_DEL(copies, entry);
        free(entry);
    }
    return out;
}
