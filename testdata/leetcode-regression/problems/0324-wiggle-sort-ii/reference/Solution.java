import java.util.*;

class Solution {
  public void wiggleSort(int[] nums) {
    int[] sorted = nums.clone();
    Arrays.sort(sorted);
    int l = (nums.length - 1) / 2, r = nums.length - 1;
    for (int i = 0; i < nums.length; i++) nums[i] = sorted[(i & 1) == 0 ? l-- : r--];
  }
}
