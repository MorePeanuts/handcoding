"use strict";
function gameOfLife(board) {
  const m = board.length,
    n = board[0]?.length || 0;
  for (let r = 0; r < m; r++)
    for (let c = 0; c < n; c++) {
      let alive = 0;
      for (let dr = -1; dr <= 1; dr++)
        for (let dc = -1; dc <= 1; dc++)
          if (
            (dr || dc) &&
            r + dr >= 0 &&
            r + dr < m &&
            c + dc >= 0 &&
            c + dc < n
          )
            alive += board[r + dr][c + dc] & 1;
      const old = board[r][c] & 1;
      if (alive === 3 || (old && alive === 2)) board[r][c] |= 2;
    }
  for (const row of board) for (let c = 0; c < n; c++) row[c] >>= 1;
}
