class Solution {
   public:
    void solve(vector<vector<char>>& b) {
        if (b.empty() || b[0].empty()) return;
        int m = b.size(), n = b[0].size();
        queue<pair<int, int>> q;
        auto add = [&](int i, int j) {
            if (i >= 0 && i < m && j >= 0 && j < n && b[i][j] == 'O') {
                b[i][j] = '#';
                q.push({i, j});
            }
        };
        for (int i = 0; i < m; ++i) {
            add(i, 0);
            add(i, n - 1);
        }
        for (int j = 0; j < n; ++j) {
            add(0, j);
            add(m - 1, j);
        }
        while (!q.empty()) {
            auto [i, j] = q.front();
            q.pop();
            add(i - 1, j);
            add(i + 1, j);
            add(i, j - 1);
            add(i, j + 1);
        }
        for (auto& row : b)
            for (auto& c : row) c = c == '#' ? 'O' : 'X';
    }
};
