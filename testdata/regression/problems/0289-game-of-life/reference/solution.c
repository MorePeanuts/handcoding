void gameOfLife(int** board, int boardSize, int* boardColSize) {
    if (!boardSize) return;
    int m = boardSize, n = boardColSize[0];
    for (int i = 0; i < m; ++i)
        for (int j = 0; j < n; ++j) {
            int count = 0;
            for (int x = i - 1; x <= i + 1; ++x)
                for (int y = j - 1; y <= j + 1; ++y)
                    if (x >= 0 && x < m && y >= 0 && y < n &&
                        (x != i || y != j))
                        count += board[x][y] & 1;
            if (count == 3 || ((board[i][j] & 1) && count == 2))
                board[i][j] |= 2;
        }
    for (int i = 0; i < m; ++i)
        for (int j = 0; j < n; ++j) board[i][j] >>= 1;
}
