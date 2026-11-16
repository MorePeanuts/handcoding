class Solution {
   public:
    TreeNode* lowestCommonAncestor(TreeNode* root, TreeNode* p, TreeNode* q) {
        unordered_map<TreeNode*, TreeNode*> parent{{root, nullptr}};
        vector<TreeNode*> todo{root};
        while (!parent.count(p) || !parent.count(q)) {
            auto n = todo.back();
            todo.pop_back();
            for (auto c : {n->left, n->right})
                if (c) {
                    parent[c] = n;
                    todo.push_back(c);
                }
        }
        unordered_set<TreeNode*> ancestors;
        for (auto n = p; n; n = parent[n]) ancestors.insert(n);
        while (!ancestors.count(q)) q = parent[q];
        return q;
    }
};
