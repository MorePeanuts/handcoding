import java.util.*;

class Solution {
  public int[] twoSum(int[] nums, int target) {
    Map<Integer, Integer> seen = new HashMap<>();
    for (int i = 0; i < nums.length; i++) {
      Integer j = seen.get(target - nums[i]);
      if (j != null)
        return new int[] {j, i};
      seen.put(nums[i], i);
    }
    return new int[0];
  }
}
