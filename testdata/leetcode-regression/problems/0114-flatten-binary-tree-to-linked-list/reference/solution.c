void flatten(struct TreeNode* root) {
    for (struct TreeNode* n = root; n; n = n->right)
        if (n->left) {
            struct TreeNode* tail = n->left;
            while (tail->right) tail = tail->right;
            tail->right = n->right;
            n->right = n->left;
            n->left = NULL;
        }
}
