function findOrder(numCourses: number, prerequisites: number[][]): number[] {
  const graph: number[][] = Array.from({ length: numCourses }, () => []),
    degree = new Array(numCourses).fill(0);
  for (const [a, b] of prerequisites) {
    graph[b].push(a);
    degree[a]++;
  }
  const q: number[] = [];
  for (let i = 0; i < numCourses; i++) if (!degree[i]) q.push(i);
  for (let i = 0; i < q.length; i++)
    for (const n of graph[q[i]]) if (--degree[n] === 0) q.push(n);
  return q.length === numCourses ? q : [];
}
