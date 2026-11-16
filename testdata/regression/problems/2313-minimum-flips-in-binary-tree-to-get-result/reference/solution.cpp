class Solution {
   public:
    int minimumFlips(TreeNode* root, bool result) {
        unordered_map<TreeNode*, array<int, 2>> cost;
        vector<pair<TreeNode*, bool>> todo{{root, false}};
        while (!todo.empty()) {
            auto [n, done] = todo.back();
            todo.pop_back();
            if (!done) {
                todo.push_back({n, true});
                if (n->left) todo.push_back({n->left, false});
                if (n->right) todo.push_back({n->right, false});
                continue;
            }
            if (n->val < 2) {
                cost[n] = {n->val == 0 ? 0 : 1, n->val == 1 ? 0 : 1};
                continue;
            }
            if (n->val == 5) {
                auto c = n->left ? n->left : n->right;
                cost[n] = {cost[c][1], cost[c][0]};
                continue;
            }
            cost[n] = {1000000, 1000000};
            for (int a = 0; a < 2; ++a)
                for (int b = 0; b < 2; ++b) {
                    int v = n->val == 2   ? (a | b)
                            : n->val == 3 ? (a & b)
                                          : (a ^ b);
                    cost[n][v] =
                        min(cost[n][v], cost[n->left][a] + cost[n->right][b]);
                }
        }
        return cost[root][result];
    }
};
