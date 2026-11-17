function solve(board: string[][]): void {
  if (!board.length || !board[0].length) return;
  const m = board.length,
    n = board[0].length,
    stack: number[][] = [];
  function mark(r: number, c: number) {
    if (r >= 0 && r < m && c >= 0 && c < n && board[r][c] === "O") {
      board[r][c] = "#";
      stack.push([r, c]);
    }
  }
  for (let r = 0; r < m; r++) {
    mark(r, 0);
    mark(r, n - 1);
  }
  for (let c = 0; c < n; c++) {
    mark(0, c);
    mark(m - 1, c);
  }
  while (stack.length) {
    const [r, c] = stack.pop();
    mark(r - 1, c);
    mark(r + 1, c);
    mark(r, c - 1);
    mark(r, c + 1);
  }
  for (const row of board)
    for (let c = 0; c < n; c++) row[c] = row[c] === "#" ? "O" : "X";
}
