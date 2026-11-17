typedef struct Cost {
    struct TreeNode* node;
    int value[2];
    UT_hash_handle hh;
} Cost;
typedef struct {
    struct TreeNode* node;
    bool done;
} Frame;
int minimumFlips(struct TreeNode* root, bool result) {
    int size = 0, capacity = 64;
    Frame* todo = malloc((size_t)capacity * sizeof(*todo));
    todo[size++] = (Frame){root, false};
    Cost *costs = NULL, *entry, *left, *right, *tmp;
    while (size) {
        Frame f = todo[--size];
        struct TreeNode* n = f.node;
        if (!f.done) {
            if (size + 3 > capacity) {
                capacity *= 2;
                todo = realloc(todo, (size_t)capacity * sizeof(*todo));
            }
            todo[size++] = (Frame){n, true};
            if (n->left) todo[size++] = (Frame){n->left, false};
            if (n->right) todo[size++] = (Frame){n->right, false};
            continue;
        }
        entry = malloc(sizeof(*entry));
        entry->node = n;
        if (n->val < 2) {
            entry->value[0] = n->val != 0;
            entry->value[1] = n->val != 1;
        } else if (n->val == 5) {
            struct TreeNode* child = n->left ? n->left : n->right;
            HASH_FIND_PTR(costs, &child, left);
            entry->value[0] = left->value[1];
            entry->value[1] = left->value[0];
        } else {
            HASH_FIND_PTR(costs, &n->left, left);
            HASH_FIND_PTR(costs, &n->right, right);
            entry->value[0] = entry->value[1] = INT_MAX;
            for (int a = 0; a < 2; ++a)
                for (int b = 0; b < 2; ++b) {
                    int value = n->val == 2   ? (a | b)
                                : n->val == 3 ? (a & b)
                                              : (a ^ b);
                    int total = left->value[a] + right->value[b];
                    if (total < entry->value[value])
                        entry->value[value] = total;
                }
        }
        HASH_ADD_PTR(costs, node, entry);
    }
    HASH_FIND_PTR(costs, &root, entry);
    int answer = entry->value[result];
    HASH_ITER(hh, costs, entry, tmp) {
        HASH_DEL(costs, entry);
        free(entry);
    }
    free(todo);
    return answer;
}
