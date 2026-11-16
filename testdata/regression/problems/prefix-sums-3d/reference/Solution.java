import java.util.*;

class Solution {
  public int[][][] prefixSums(int[][][] cube) {
    int[][][] a = new int[cube.length][][];
    for (int i = 0; i < cube.length; i++) {
      a[i] = new int[cube[i].length][];
      for (int j = 0; j < cube[i].length; j++) {
        a[i][j] = new int[cube[i][j].length];
        for (int k = 0; k < a[i][j].length; k++)
          a[i][j][k] = cube[i][j][k] + at(a, i - 1, j, k) + at(a, i, j - 1, k) + at(a, i, j, k - 1)
              - at(a, i - 1, j - 1, k) - at(a, i - 1, j, k - 1) - at(a, i, j - 1, k - 1)
              + at(a, i - 1, j - 1, k - 1);
      }
    }
    return a;
  }
  int at(int[][][] a, int i, int j, int k) {
    return i < 0 || j < 0 || k < 0 ? 0 : a[i][j][k];
  }
}
