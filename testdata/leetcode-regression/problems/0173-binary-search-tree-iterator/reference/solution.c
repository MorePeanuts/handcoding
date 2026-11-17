typedef struct {
    struct TreeNode** stack;
    int size, capacity;
} BSTIterator;
static void descend(BSTIterator* obj, struct TreeNode* n) {
    while (n) {
        if (obj->size == obj->capacity) {
            obj->capacity *= 2;
            obj->stack = realloc(obj->stack,
                                 (size_t)obj->capacity * sizeof(*obj->stack));
        }
        obj->stack[obj->size++] = n;
        n = n->left;
    }
}
BSTIterator* bSTIteratorCreate(struct TreeNode* root) {
    BSTIterator* obj = calloc(1, sizeof(*obj));
    obj->capacity = 16;
    obj->stack = malloc((size_t)obj->capacity * sizeof(*obj->stack));
    descend(obj, root);
    return obj;
}
int bSTIteratorNext(BSTIterator* obj) {
    struct TreeNode* n = obj->stack[--obj->size];
    descend(obj, n->right);
    return n->val;
}
bool bSTIteratorHasNext(BSTIterator* obj) { return obj->size != 0; }
void bSTIteratorFree(BSTIterator* obj) {
    free(obj->stack);
    free(obj);
}
