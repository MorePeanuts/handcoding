import java.util.*;

class Solution {
  public List<List<Integer>> permute(int[] nums) {
    List<List<Integer>> out = new ArrayList<>();
    visit(nums, new boolean[nums.length], new ArrayList<>(), out);
    return out;
  }
  void visit(int[] nums, boolean[] used, List<Integer> path, List<List<Integer>> out) {
    if (path.size() == nums.length) {
      out.add(new ArrayList<>(path));
      return;
    }
    for (int i = 0; i < nums.length; i++)
      if (!used[i]) {
        used[i] = true;
        path.add(nums[i]);
        visit(nums, used, path, out);
        path.remove(path.size() - 1);
        used[i] = false;
      }
  }
}
