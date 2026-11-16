import java.util.*;

class Solution {
  public double findMedianSortedArrays(int[] a, int[] b) {
    if (a.length > b.length)
      return findMedianSortedArrays(b, a);
    int lo = 0, hi = a.length;
    while (lo <= hi) {
      int i = (lo + hi) / 2, j = (a.length + b.length + 1) / 2 - i;
      int al = i == 0 ? Integer.MIN_VALUE : a[i - 1], ar = i == a.length ? Integer.MAX_VALUE : a[i],
          bl = j == 0 ? Integer.MIN_VALUE : b[j - 1], br = j == b.length ? Integer.MAX_VALUE : b[j];
      if (al <= br && bl <= ar)
        return (a.length + b.length) % 2 == 1 ? Math.max(al, bl)
                                              : ((double) Math.max(al, bl) + Math.min(ar, br)) / 2;
      if (al > br)
        hi = i - 1;
      else
        lo = i + 1;
    }
    throw new IllegalArgumentException();
  }
}
