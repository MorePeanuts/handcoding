class Solution {
   public:
    int maxDepth(TreeNode* root) {
        vector<pair<TreeNode*, int>> todo;
        if (root) todo.push_back({root, 1});
        int ans = 0;
        while (!todo.empty()) {
            auto [n, d] = todo.back();
            todo.pop_back();
            ans = max(ans, d);
            if (n->left) todo.push_back({n->left, d + 1});
            if (n->right) todo.push_back({n->right, d + 1});
        }
        return ans;
    }
};
