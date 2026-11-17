import java.util.*;

class Solution {
  public void solve(char[][] b) {
    if (b.length == 0 || b[0].length == 0)
      return;
    int m = b.length, n = b[0].length;
    Deque<int[]> q = new ArrayDeque<>();
    for (int i = 0; i < m; i++) {
      mark(b, i, 0, q);
      mark(b, i, n - 1, q);
    }
    for (int j = 0; j < n; j++) {
      mark(b, 0, j, q);
      mark(b, m - 1, j, q);
    }
    while (!q.isEmpty()) {
      int[] x = q.remove();
      for (int[] d : new int[][] {{1, 0}, {-1, 0}, {0, 1}, {0, -1}})
        mark(b, x[0] + d[0], x[1] + d[1], q);
    }
    for (char[] row : b)
      for (int j = 0; j < n; j++) row[j] = row[j] == '#' ? 'O' : 'X';
  }
  void mark(char[][] b, int i, int j, Deque<int[]> q) {
    if (i >= 0 && j >= 0 && i < b.length && j < b[0].length && b[i][j] == 'O') {
      b[i][j] = '#';
      q.add(new int[] {i, j});
    }
  }
}
