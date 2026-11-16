import java.util.*;

class Solution {
  public NestedInteger deserialize(String s) {
    if (s.charAt(0) != '[')
      return new NestedInteger(Integer.parseInt(s));
    Deque<NestedInteger> stack = new ArrayDeque<>();
    NestedInteger root = null;
    for (int i = 0; i < s.length();) {
      char c = s.charAt(i);
      if (c == '[') {
        NestedInteger n = new NestedInteger();
        if (!stack.isEmpty())
          stack.peek().add(n);
        else
          root = n;
        stack.push(n);
        i++;
      } else if (c == ']') {
        stack.pop();
        i++;
      } else if (c == ',')
        i++;
      else {
        int j = i++;
        while (i < s.length() && Character.isDigit(s.charAt(i))) i++;
        stack.peek().add(new NestedInteger(Integer.parseInt(s.substring(j, i))));
      }
    }
    return root;
  }
}
