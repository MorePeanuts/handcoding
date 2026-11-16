class BSTIterator {
    vector<TreeNode*> todo;
    void descend(TreeNode* n) {
        while (n) {
            todo.push_back(n);
            n = n->left;
        }
    }

   public:
    BSTIterator(TreeNode* root) { descend(root); }
    int next() {
        auto n = todo.back();
        todo.pop_back();
        descend(n->right);
        return n->val;
    }
    bool hasNext() { return !todo.empty(); }
};
