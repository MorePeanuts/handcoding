static void add(char** board, int m, int n, int row, int col, int* queue,
                int* end) {
    if (row < 0 || row >= m || col < 0 || col >= n || board[row][col] != 'O')
        return;
    board[row][col] = '#';
    queue[(*end)++] = row * n + col;
}
void solve(char** board, int boardSize, int* boardColSize) {
    if (!boardSize || !boardColSize[0]) return;
    int m = boardSize, n = boardColSize[0], end = 0;
    int* queue = malloc((size_t)m * (size_t)n * sizeof(int));
    for (int i = 0; i < m; ++i) {
        add(board, m, n, i, 0, queue, &end);
        add(board, m, n, i, n - 1, queue, &end);
    }
    for (int j = 0; j < n; ++j) {
        add(board, m, n, 0, j, queue, &end);
        add(board, m, n, m - 1, j, queue, &end);
    }
    for (int at = 0; at < end; ++at) {
        int row = queue[at] / n, col = queue[at] % n;
        add(board, m, n, row - 1, col, queue, &end);
        add(board, m, n, row + 1, col, queue, &end);
        add(board, m, n, row, col - 1, queue, &end);
        add(board, m, n, row, col + 1, queue, &end);
    }
    for (int i = 0; i < m; ++i)
        for (int j = 0; j < n; ++j)
            board[i][j] = board[i][j] == '#' ? 'O' : 'X';
    free(queue);
}
