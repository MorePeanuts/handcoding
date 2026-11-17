int*** prefixSums(int*** grid, int gridSize, int* gridColSize,
                  int** gridDepthSizes, int* returnSize,
                  int** returnColumnSizes, int*** returnDepthSizes) {
    /* The hc 3D ABI carries both ragged dimensions independently. */
    *returnSize = gridSize;
    *returnColumnSizes = gridColSize;
    *returnDepthSizes = gridDepthSizes;
    for (int i = 0; i < gridSize; ++i)
        for (int j = 0; j < gridColSize[i]; ++j)
            for (int k = 0; k < gridDepthSizes[i][j]; ++k) {
                if (i) grid[i][j][k] += grid[i - 1][j][k];
                if (j) grid[i][j][k] += grid[i][j - 1][k];
                if (k) grid[i][j][k] += grid[i][j][k - 1];
                if (i && j) grid[i][j][k] -= grid[i - 1][j - 1][k];
                if (i && k) grid[i][j][k] -= grid[i - 1][j][k - 1];
                if (j && k) grid[i][j][k] -= grid[i][j - 1][k - 1];
                if (i && j && k) grid[i][j][k] += grid[i - 1][j - 1][k - 1];
            }
    return grid;
}
