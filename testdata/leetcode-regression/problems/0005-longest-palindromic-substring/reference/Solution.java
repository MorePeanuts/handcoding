import java.util.*;

class Solution {
  public String longestPalindrome(String s) {
    int start = 0, length = 0;
    for (int c = 0; c < 2 * s.length(); c++) {
      int l = c / 2, r = l + c % 2;
      while (l >= 0 && r < s.length() && s.charAt(l) == s.charAt(r)) {
        if (r - l + 1 > length) {
          start = l;
          length = r - l + 1;
        }
        l--;
        r++;
      }
    }
    return s.substring(start, start + length);
  }
}
