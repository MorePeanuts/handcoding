class Solution {
   public:
    int numIslands(vector<vector<char>>& grid) {
        if (grid.empty()) return 0;
        int m = grid.size(), n = grid[0].size(), answer = 0;
        for (int i = 0; i < m; ++i)
            for (int j = 0; j < n; ++j)
                if (grid[i][j] == '1') {
                    ++answer;
                    vector<pair<int, int>> todo{{i, j}};
                    grid[i][j] = '0';
                    while (!todo.empty()) {
                        auto [x, y] = todo.back();
                        todo.pop_back();
                        for (auto [dx, dy] : vector<pair<int, int>>{
                                 {1, 0}, {-1, 0}, {0, 1}, {0, -1}}) {
                            int a = x + dx, b = y + dy;
                            if (a >= 0 && a < m && b >= 0 && b < n &&
                                grid[a][b] == '1') {
                                grid[a][b] = '0';
                                todo.push_back({a, b});
                            }
                        }
                    }
                }
        return answer;
    }
};
