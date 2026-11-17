"use strict";
function prefixSums(grid) {
  const a = grid.length,
    b = grid[0]?.length || 0,
    c = grid[0]?.[0]?.length || 0;
  const out = grid.map((plane) => plane.map((row) => row.slice()));
  for (let i = 0; i < a; i++)
    for (let j = 0; j < b; j++)
      for (let k = 0; k < c; k++) {
        if (i) out[i][j][k] += out[i - 1][j][k];
        if (j) out[i][j][k] += out[i][j - 1][k];
        if (k) out[i][j][k] += out[i][j][k - 1];
        if (i && j) out[i][j][k] -= out[i - 1][j - 1][k];
        if (i && k) out[i][j][k] -= out[i - 1][j][k - 1];
        if (j && k) out[i][j][k] -= out[i][j - 1][k - 1];
        if (i && j && k) out[i][j][k] += out[i - 1][j - 1][k - 1];
      }
  return out;
}
