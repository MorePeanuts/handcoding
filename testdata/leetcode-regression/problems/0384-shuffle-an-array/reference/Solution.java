import java.util.*;

class Solution {
  final int[] original;
  final Random random = new Random();
  public Solution(int[] nums) {
    original = nums.clone();
  }
  public int[] reset() {
    return original.clone();
  }
  public int[] shuffle() {
    int[] a = original.clone();
    for (int i = a.length - 1; i > 0; i--) {
      int j = random.nextInt(i + 1), t = a[i];
      a[i] = a[j];
      a[j] = t;
    }
    return a;
  }
}
