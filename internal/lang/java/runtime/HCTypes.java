import java.util.*;

class ListNode {
  int val;
  ListNode next;
  ListNode() {}
  ListNode(int val) {
    this.val = val;
  }
  ListNode(int val, ListNode next) {
    this.val = val;
    this.next = next;
  }
}
class TreeNode {
  int val;
  TreeNode left, right;
  TreeNode() {}
  TreeNode(int val) {
    this.val = val;
  }
  TreeNode(int val, TreeNode left, TreeNode right) {
    this.val = val;
    this.left = left;
    this.right = right;
  }
}
class NestedInteger {
  private Integer value;
  private final List<NestedInteger> list = new ArrayList<>();
  public NestedInteger() {}
  public NestedInteger(int value) {
    this.value = value;
  }
  public boolean isInteger() {
    return value != null;
  }
  public Integer getInteger() {
    return value;
  }
  public void setInteger(int value) {
    this.value = value;
    list.clear();
  }
  public void add(NestedInteger n) {
    value = null;
    list.add(n);
  }
  public List<NestedInteger> getList() {
    return value == null ? list : null;
  }
}
class Employee {
  public int id, importance;
  public List<Integer> subordinates = new ArrayList<>();
}
