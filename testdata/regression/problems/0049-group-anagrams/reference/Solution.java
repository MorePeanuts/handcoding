import java.util.*;

class Solution {
  public List<List<String>> groupAnagrams(String[] strs) {
    Map<String, List<String>> groups = new LinkedHashMap<>();
    for (String s : strs) {
      char[] key = s.toCharArray();
      Arrays.sort(key);
      groups.computeIfAbsent(new String(key), k -> new ArrayList<>()).add(s);
    }
    return new ArrayList<>(groups.values());
  }
}
