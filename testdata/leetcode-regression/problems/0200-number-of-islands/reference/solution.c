int numIslands(char** grid, int gridSize, int* gridColSize) {
    if (!gridSize || !gridColSize[0]) return 0;
    int m = gridSize, n = gridColSize[0], answer = 0;
    int* todo = malloc((size_t)m * (size_t)n * sizeof(int));
    const int dx[] = {1, -1, 0, 0}, dy[] = {0, 0, 1, -1};
    for (int i = 0; i < m; ++i)
        for (int j = 0; j < n; ++j)
            if (grid[i][j] == '1') {
                ++answer;
                int size = 0;
                todo[size++] = i * n + j;
                grid[i][j] = '0';
                while (size) {
                    int cell = todo[--size], x = cell / n, y = cell % n;
                    for (int d = 0; d < 4; ++d) {
                        int a = x + dx[d], b = y + dy[d];
                        if (a >= 0 && a < m && b >= 0 && b < n &&
                            grid[a][b] == '1') {
                            grid[a][b] = '0';
                            todo[size++] = a * n + b;
                        }
                    }
                }
            }
    free(todo);
    return answer;
}
