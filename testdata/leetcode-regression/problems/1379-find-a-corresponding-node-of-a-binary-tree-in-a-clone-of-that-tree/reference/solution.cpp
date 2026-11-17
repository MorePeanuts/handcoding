class Solution {
   public:
    TreeNode* getTargetCopy(TreeNode* original, TreeNode* cloned,
                            TreeNode* target) {
        vector<pair<TreeNode*, TreeNode*>> todo{{original, cloned}};
        while (!todo.empty()) {
            auto [a, b] = todo.back();
            todo.pop_back();
            if (a == target) return b;
            if (a->left) todo.push_back({a->left, b->left});
            if (a->right) todo.push_back({a->right, b->right});
        }
        return nullptr;
    }
};
