import java.util.*;

class Solution {
  public int[] findOrder(int n, int[][] edges) {
    List<List<Integer>> adj = new ArrayList<>();
    for (int i = 0; i < n; i++) adj.add(new ArrayList<>());
    int[] degree = new int[n], out = new int[n];
    for (int[] e : edges) {
      adj.get(e[1]).add(e[0]);
      degree[e[0]]++;
    }
    Deque<Integer> q = new ArrayDeque<>();
    for (int i = 0; i < n; i++)
      if (degree[i] == 0)
        q.add(i);
    int k = 0;
    while (!q.isEmpty()) {
      int v = q.remove();
      out[k++] = v;
      for (int u : adj.get(v))
        if (--degree[u] == 0)
          q.add(u);
    }
    return k == n ? out : new int[0];
  }
}
