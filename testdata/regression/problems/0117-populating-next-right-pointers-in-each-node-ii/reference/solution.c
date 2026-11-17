struct Node* connect(struct Node* root) {
    if (root) root->next = NULL;
    for (struct Node* level = root; level;) {
        struct Node dummy = {0}, *tail = &dummy;
        for (struct Node* n = level; n; n = n->next) {
            if (n->left) {
                tail->next = n->left;
                tail = tail->next;
            }
            if (n->right) {
                tail->next = n->right;
                tail = tail->next;
            }
        }
        tail->next = NULL;
        level = dummy.next;
    }
    return root;
}
