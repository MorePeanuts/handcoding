import java.util.*;

class Solution {
  public char nextGreatestLetter(char[] letters, char target) {
    int l = 0, r = letters.length;
    while (l < r) {
      int m = (l + r) / 2;
      if (letters[m] <= target)
        l = m + 1;
      else
        r = m;
    }
    return letters[l % letters.length];
  }
}
