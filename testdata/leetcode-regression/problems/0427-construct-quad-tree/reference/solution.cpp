class Solution {
   public:
    Node* build(vector<vector<int>>& a, int x, int y, int n) {
        bool same = true;
        for (int i = x; i < x + n; ++i)
            for (int j = y; j < y + n; ++j)
                if (a[i][j] != a[x][y]) same = false;
        if (same) return new Node(a[x][y], true);
        int k = n / 2;
        return new Node(true, false, build(a, x, y, k), build(a, x, y + k, k),
                        build(a, x + k, y, k), build(a, x + k, y + k, k));
    }
    Node* construct(vector<vector<int>>& grid) {
        return build(grid, 0, 0, grid.size());
    }
};
