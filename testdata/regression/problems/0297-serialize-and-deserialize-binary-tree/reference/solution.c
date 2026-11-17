/* Breadth-first tokens with explicit nulls; decoding has no shared state. */
char* serialize(struct TreeNode* root) {
    int capacity = 64, size = 1;
    struct TreeNode** queue = malloc((size_t)capacity * sizeof(*queue));
    queue[0] = root;
    size_t length = 0, bytes = 128;
    char* out = malloc(bytes);
    for (int i = 0; i < size; ++i) {
        struct TreeNode* n = queue[i];
        if (length + 32 > bytes) {
            bytes *= 2;
            out = realloc(out, bytes);
        }
        if (!n) {
            out[length++] = '#';
            out[length++] = ' ';
            continue;
        }
        length += (size_t)sprintf(out + length, "%d ", n->val);
        if (size + 2 > capacity) {
            capacity *= 2;
            queue = realloc(queue, (size_t)capacity * sizeof(*queue));
        }
        queue[size++] = n->left;
        queue[size++] = n->right;
    }
    out[length] = '\0';
    free(queue);
    return out;
}
static struct TreeNode* readNode(char** data) {
    if (**data == '#') {
        *data += 2;
        return NULL;
    }
    struct TreeNode* n = calloc(1, sizeof(*n));
    n->val = (int)strtol(*data, data, 10);
    ++*data;
    return n;
}
struct TreeNode* deserialize(char* data) {
    struct TreeNode* root = readNode(&data);
    if (!root) return NULL;
    int capacity = 64, size = 1;
    struct TreeNode** queue = malloc((size_t)capacity * sizeof(*queue));
    queue[0] = root;
    for (int i = 0; i < size; ++i) {
        struct TreeNode* n = queue[i];
        n->left = readNode(&data);
        n->right = readNode(&data);
        if (size + 2 > capacity) {
            capacity *= 2;
            queue = realloc(queue, (size_t)capacity * sizeof(*queue));
        }
        if (n->left) queue[size++] = n->left;
        if (n->right) queue[size++] = n->right;
    }
    free(queue);
    return root;
}
