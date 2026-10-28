class Solution:
    def solve(self, board: list[list[str]]) -> None:
        m, n = len(board), len(board[0])
        # Mark every 'O' reachable from the border, then capture the rest.
        stack = [(i, j) for i in range(m) for j in (0, n - 1)]
        stack += [(i, j) for i in (0, m - 1) for j in range(n)]
        while stack:
            i, j = stack.pop()
            if 0 <= i < m and 0 <= j < n and board[i][j] == "O":
                board[i][j] = "#"
                stack += [(i + 1, j), (i - 1, j), (i, j + 1), (i, j - 1)]
        for row in board:
            for j, c in enumerate(row):
                row[j] = "O" if c == "#" else "X"
