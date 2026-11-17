static struct Node* build(int** grid, int row, int col, int size) {
    bool same = true;
    for (int i = row; i < row + size; ++i)
        for (int j = col; j < col + size; ++j)
            if (grid[i][j] != grid[row][col]) same = false;
    struct Node* out = calloc(1, sizeof(*out));
    out->isLeaf = same;
    out->val = same ? grid[row][col] != 0 : true;
    if (!same) {
        int half = size / 2;
        out->topLeft = build(grid, row, col, half);
        out->topRight = build(grid, row, col + half, half);
        out->bottomLeft = build(grid, row + half, col, half);
        out->bottomRight = build(grid, row + half, col + half, half);
    }
    return out;
}
struct Node* construct(int** grid, int gridSize, int* gridColSize) {
    (void)gridColSize;
    return gridSize ? build(grid, 0, 0, gridSize) : NULL;
}
