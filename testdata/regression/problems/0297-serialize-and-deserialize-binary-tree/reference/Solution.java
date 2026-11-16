import java.util.*;

class Codec {
  public String serialize(TreeNode root) {
    StringBuilder out = new StringBuilder();
    List<TreeNode> q = new ArrayList<>();
    q.add(root);
    for (int i = 0; i < q.size(); i++) {
      TreeNode n = q.get(i);
      if (n == null)
        out.append("#,");
      else {
        out.append(n.val).append(',');
        q.add(n.left);
        q.add(n.right);
      }
    }
    return out.toString();
  }
  public TreeNode deserialize(String data) {
    String[] a = data.split(",");
    if (a[0].equals("#"))
      return null;
    TreeNode root = new TreeNode(Integer.parseInt(a[0]));
    Deque<TreeNode> q = new ArrayDeque<>();
    q.add(root);
    int i = 1;
    while (!q.isEmpty()) {
      TreeNode n = q.remove();
      if (!a[i].equals("#")) {
        n.left = new TreeNode(Integer.parseInt(a[i]));
        q.add(n.left);
      }
      i++;
      if (!a[i].equals("#")) {
        n.right = new TreeNode(Integer.parseInt(a[i]));
        q.add(n.right);
      }
      i++;
    }
    return root;
  }
}
