class Solution {
   public:
    void gameOfLife(vector<vector<int>>& board) {
        if (board.empty()) return;
        auto old = board;
        int m = board.size(), n = board[0].size();
        for (int i = 0; i < m; ++i)
            for (int j = 0; j < n; ++j) {
                int neighbors = 0;
                for (int x = max(0, i - 1); x <= min(m - 1, i + 1); ++x)
                    for (int y = max(0, j - 1); y <= min(n - 1, j + 1); ++y)
                        if (x != i || y != j) neighbors += old[x][y];
                board[i][j] = neighbors == 3 || (old[i][j] && neighbors == 2);
            }
    }
};
