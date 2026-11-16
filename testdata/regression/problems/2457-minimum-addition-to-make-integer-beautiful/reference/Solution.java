import java.util.*;

class Solution {
  public long makeIntegerBeautiful(long n, int target) {
    long x = n, place = 1;
    while (sum(x) > target) {
      x = (x / place + 1) * place;
      place *= 10;
    }
    return x - n;
  }
  int sum(long x) {
    int out = 0;
    while (x > 0) {
      out += x % 10;
      x /= 10;
    }
    return out;
  }
}
