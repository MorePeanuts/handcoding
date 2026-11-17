typedef struct {
    struct TreeNode** trees;
    int size, capacity;
} Trees;
static Trees build(int lo, int hi) {
    Trees out = {malloc(sizeof(struct TreeNode*)), 0, 1};
    if (lo > hi) {
        out.trees[out.size++] = NULL;
        return out;
    }
    for (int value = lo; value <= hi; ++value) {
        Trees left = build(lo, value - 1), right = build(value + 1, hi);
        for (int i = 0; i < left.size; ++i)
            for (int j = 0; j < right.size; ++j) {
                if (out.size == out.capacity) {
                    out.capacity *= 2;
                    out.trees = realloc(
                        out.trees, (size_t)out.capacity * sizeof(*out.trees));
                }
                struct TreeNode* n = malloc(sizeof(*n));
                *n = (struct TreeNode){value, left.trees[i], right.trees[j]};
                out.trees[out.size++] = n;
            }
        free(left.trees);
        free(right.trees);
    }
    return out;
}
struct TreeNode** generateTrees(int n, int* returnSize) {
    if (!n) {
        *returnSize = 0;
        return NULL;
    }
    Trees out = build(1, n);
    *returnSize = out.size;
    return out.trees;
}
