import java.util.*;

class NestedIterator implements Iterator<Integer> {
  final Deque<Iterator<NestedInteger>> stack = new ArrayDeque<>();
  Integer pending;
  public NestedIterator(List<NestedInteger> nestedList) {
    stack.push(nestedList.iterator());
  }
  public Integer next() {
    if (!hasNext())
      throw new NoSuchElementException();
    Integer n = pending;
    pending = null;
    return n;
  }
  public boolean hasNext() {
    if (pending != null)
      return true;
    while (!stack.isEmpty()) {
      if (!stack.peek().hasNext()) {
        stack.pop();
        continue;
      }
      NestedInteger n = stack.peek().next();
      if (n.isInteger()) {
        pending = n.getInteger();
        return true;
      }
      stack.push(n.getList().iterator());
    }
    return false;
  }
}
