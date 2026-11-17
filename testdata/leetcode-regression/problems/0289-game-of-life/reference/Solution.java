import java.util.*;

class Solution {
  public void gameOfLife(int[][] b) {
    int m = b.length;
    if (m == 0)
      return;
    int n = b[0].length;
    for (int i = 0; i < m; i++)
      for (int j = 0; j < n; j++) {
        int count = 0;
        for (int di = -1; di <= 1; di++)
          for (int dj = -1; dj <= 1; dj++)
            if ((di != 0 || dj != 0) && i + di >= 0 && i + di < m && j + dj >= 0 && j + dj < n)
              count += b[i + di][j + dj] & 1;
        if (count == 3 || count == 2 && (b[i][j] & 1) == 1)
          b[i][j] |= 2;
      }
    for (int[] row : b)
      for (int j = 0; j < n; j++) row[j] >>= 1;
  }
}
