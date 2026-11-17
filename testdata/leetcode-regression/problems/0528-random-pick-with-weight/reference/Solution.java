import java.util.*;

class Solution {
  final long[] sums;
  final Random random = new Random();
  public Solution(int[] w) {
    sums = new long[w.length];
    for (int i = 0; i < w.length; i++) sums[i] = w[i] + (i == 0 ? 0 : sums[i - 1]);
  }
  public int pickIndex() {
    long x = random.nextLong(sums[sums.length - 1]);
    int l = 0, r = sums.length - 1;
    while (l < r) {
      int m = (l + r) / 2;
      if (sums[m] > x)
        r = m;
      else
        l = m + 1;
    }
    return l;
  }
}
