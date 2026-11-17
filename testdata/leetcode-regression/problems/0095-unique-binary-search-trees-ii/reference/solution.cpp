class Solution {
   public:
    vector<TreeNode*> build(int l, int r) {
        if (l > r) return {nullptr};
        vector<TreeNode*> out;
        for (int i = l; i <= r; ++i)
            for (auto a : build(l, i - 1))
                for (auto b : build(i + 1, r))
                    out.push_back(new TreeNode(i, a, b));
        return out;
    }
    vector<TreeNode*> generateTrees(int n) {
        return n ? build(1, n) : vector<TreeNode*>{};
    }
};
