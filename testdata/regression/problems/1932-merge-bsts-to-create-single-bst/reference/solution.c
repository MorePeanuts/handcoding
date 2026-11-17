typedef struct Entry {
    int value;
    bool leaf, used;
    struct TreeNode* root;
    UT_hash_handle hh;
} Entry;
typedef struct {
    struct TreeNode* node;
    long long low, high;
} Frame;
static Entry* getEntry(Entry** entries, int value) {
    Entry* e;
    HASH_FIND_INT(*entries, &value, e);
    if (!e) {
        e = calloc(1, sizeof(*e));
        e->value = value;
        HASH_ADD_INT(*entries, value, e);
    }
    return e;
}
struct TreeNode* canMerge(struct TreeNode** trees, int treesSize) {
    Entry *entries = NULL, *e, *tmp;
    for (int i = 0; i < treesSize; ++i) {
        struct TreeNode* n = trees[i];
        getEntry(&entries, n->val)->root = n;
        if (n->left) getEntry(&entries, n->left->val)->leaf = true;
        if (n->right) getEntry(&entries, n->right->val)->leaf = true;
    }
    struct TreeNode* root = NULL;
    int candidates = 0;
    for (int i = 0; i < treesSize; ++i)
        if (!getEntry(&entries, trees[i]->val)->leaf) {
            root = trees[i];
            ++candidates;
        }
    int capacity = 64, size = 0, remaining = treesSize - 1;
    bool valid = candidates == 1;
    Frame* todo = malloc((size_t)capacity * sizeof(*todo));
    if (valid) {
        getEntry(&entries, root->val)->used = true;
        todo[size++] = (Frame){root, LLONG_MIN, LLONG_MAX};
    }
    while (valid && size) {
        Frame f = todo[--size];
        struct TreeNode* n = f.node;
        if (n->val <= f.low || n->val >= f.high) {
            valid = false;
            break;
        }
        if (!n->left && !n->right) {
            HASH_FIND_INT(entries, &n->val, e);
            if (e && e->root && !e->used) {
                n->left = e->root->left;
                n->right = e->root->right;
                e->used = true;
                --remaining;
            }
        }
        if (size + 2 > capacity) {
            capacity *= 2;
            todo = realloc(todo, (size_t)capacity * sizeof(*todo));
        }
        if (n->left) todo[size++] = (Frame){n->left, f.low, n->val};
        if (n->right) todo[size++] = (Frame){n->right, n->val, f.high};
    }
    HASH_ITER(hh, entries, e, tmp) {
        HASH_DEL(entries, e);
        free(e);
    }
    free(todo);
    return valid && !remaining ? root : NULL;
}
