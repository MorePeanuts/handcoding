class Solution:
    def gameOfLife(self, board: list[list[int]]) -> None:
        m, n = len(board), len(board[0])
        old = [row[:] for row in board]
        for i in range(m):
            for j in range(n):
                live = sum(
                    old[x][y]
                    for x in range(max(0, i - 1), min(m, i + 2))
                    for y in range(max(0, j - 1), min(n, j + 2))
                    if (x, y) != (i, j)
                )
                board[i][j] = int(live == 3 or (live == 2 and old[i][j] == 1))
