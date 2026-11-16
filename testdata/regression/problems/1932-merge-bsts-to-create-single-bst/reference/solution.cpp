class Solution {
   public:
    TreeNode* canMerge(vector<TreeNode*>& trees) {
        unordered_map<int, TreeNode*> roots;
        unordered_set<int> leaves;
        for (auto t : trees) {
            roots[t->val] = t;
            if (t->left) leaves.insert(t->left->val);
            if (t->right) leaves.insert(t->right->val);
        }
        TreeNode* root = nullptr;
        for (auto t : trees)
            if (!leaves.count(t->val)) {
                if (root) return nullptr;
                root = t;
            }
        if (!root) return nullptr;
        roots.erase(root->val);
        vector<tuple<TreeNode*, long long, long long>> todo{
            {root, LLONG_MIN, LLONG_MAX}};
        while (!todo.empty()) {
            auto [n, lo, hi] = todo.back();
            todo.pop_back();
            if (n->val <= lo || n->val >= hi) return nullptr;
            if (!n->left && !n->right && roots.count(n->val)) {
                auto t = roots[n->val];
                n->left = t->left;
                n->right = t->right;
                roots.erase(n->val);
            }
            if (n->left) todo.push_back({n->left, lo, n->val});
            if (n->right) todo.push_back({n->right, n->val, hi});
        }
        return roots.empty() ? root : nullptr;
    }
};
