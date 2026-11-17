typedef struct {
    struct TreeNode* node;
    int depth;
} Frame;
int maxDepth(struct TreeNode* root) {
    if (!root) return 0;
    int size = 0, capacity = 64, answer = 0;
    Frame* stack = malloc((size_t)capacity * sizeof(*stack));
    stack[size++] = (Frame){root, 1};
    while (size) {
        Frame f = stack[--size];
        if (f.depth > answer) answer = f.depth;
        if (size + 2 > capacity) {
            capacity *= 2;
            stack = realloc(stack, (size_t)capacity * sizeof(*stack));
        }
        if (f.node->left) stack[size++] = (Frame){f.node->left, f.depth + 1};
        if (f.node->right) stack[size++] = (Frame){f.node->right, f.depth + 1};
    }
    free(stack);
    return answer;
}
