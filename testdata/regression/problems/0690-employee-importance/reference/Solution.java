import java.util.*;

class Solution {
  public int getImportance(List<Employee> employees, int id) {
    Map<Integer, Employee> byID = new HashMap<>();
    for (Employee e : employees) byID.put(e.id, e);
    Deque<Integer> q = new ArrayDeque<>();
    q.add(id);
    int total = 0;
    while (!q.isEmpty()) {
      Employee e = byID.get(q.remove());
      total += e.importance;
      q.addAll(e.subordinates);
    }
    return total;
  }
}
