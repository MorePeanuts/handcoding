from hc_runtime import QuadNode as Node


class Solution:
    def construct(self, grid: list[list[int]]) -> Node | None:
        def build(r: int, c: int, n: int) -> Node:
            first = grid[r][c]
            if all(grid[i][j] == first for i in range(r, r + n) for j in range(c, c + n)):
                return Node(first == 1, True, None, None, None, None)
            h = n // 2
            return Node(True, False, build(r, c, h), build(r, c + h, h), build(r + h, c, h), build(r + h, c + h, h))

        return build(0, 0, len(grid))
