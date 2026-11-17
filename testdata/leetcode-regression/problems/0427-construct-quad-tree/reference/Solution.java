import java.util.*;

class Solution {
  public Node construct(int[][] grid) {
    return build(grid, 0, 0, grid.length);
  }
  Node build(int[][] g, int r, int c, int n) {
    boolean same = true;
    for (int i = r; i < r + n; i++)
      for (int j = c; j < c + n; j++)
        if (g[i][j] != g[r][c])
          same = false;
    if (same)
      return new Node(g[r][c] != 0, true);
    int h = n / 2;
    return new Node(true, false, build(g, r, c, h), build(g, r, c + h, h), build(g, r + h, c, h),
        build(g, r + h, c + h, h));
  }
}
