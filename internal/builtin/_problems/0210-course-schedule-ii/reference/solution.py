from collections import deque


class Solution:
    def findOrder(self, numCourses: int, prerequisites: list[list[int]]) -> list[int]:
        # Kahn's algorithm with a queue: courses in the order they become free.
        after = [[] for _ in range(numCourses)]
        indegree = [0] * numCourses
        for a, b in prerequisites:
            after[b].append(a)
            indegree[a] += 1
        free = deque(c for c in range(numCourses) if indegree[c] == 0)
        order = []
        while free:
            c = free.popleft()
            order.append(c)
            for a in after[c]:
                indegree[a] -= 1
                if indegree[a] == 0:
                    free.append(a)
        return order if len(order) == numCourses else []
