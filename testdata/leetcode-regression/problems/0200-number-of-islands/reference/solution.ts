function numIslands(grid: string[][]): number {
  if (!grid.length) return 0;
  const m = grid.length,
    n = grid[0].length;
  let count = 0;
  for (let r = 0; r < m; r++)
    for (let c = 0; c < n; c++)
      if (grid[r][c] === "1") {
        count++;
        grid[r][c] = "0";
        const stack = [[r, c]];
        while (stack.length) {
          const [x, y] = stack.pop();
          for (const [a, b] of [
            [x - 1, y],
            [x + 1, y],
            [x, y - 1],
            [x, y + 1],
          ])
            if (a >= 0 && a < m && b >= 0 && b < n && grid[a][b] === "1") {
              grid[a][b] = "0";
              stack.push([a, b]);
            }
        }
      }
  return count;
}
