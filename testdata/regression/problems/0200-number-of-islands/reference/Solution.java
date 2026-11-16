import java.util.*;

class Solution {
  public int numIslands(char[][] a) {
    if (a.length == 0)
      return 0;
    int count = 0, m = a.length, n = a[0].length;
    Deque<int[]> q = new ArrayDeque<>();
    for (int i = 0; i < m; i++)
      for (int j = 0; j < n; j++)
        if (a[i][j] == '1') {
          count++;
          a[i][j] = '0';
          q.add(new int[] {i, j});
          while (!q.isEmpty()) {
            int[] p = q.remove();
            for (int[] d : new int[][] {{1, 0}, {-1, 0}, {0, 1}, {0, -1}}) {
              int r = p[0] + d[0], c = p[1] + d[1];
              if (r >= 0 && r < m && c >= 0 && c < n && a[r][c] == '1') {
                a[r][c] = '0';
                q.add(new int[] {r, c});
              }
            }
          }
        }
    return count;
  }
}
