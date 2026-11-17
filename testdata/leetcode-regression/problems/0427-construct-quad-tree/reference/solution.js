"use strict";
function construct(grid) {
  function build(r, c, n) {
    const value = grid[r][c];
    let same = true;
    for (let i = r; i < r + n && same; i++)
      for (let j = c; j < c + n; j++)
        if (grid[i][j] !== value) {
          same = false;
          break;
        }
    if (same) return new _Node(!!value, true);
    const h = n / 2;
    return new _Node(
      true,
      false,
      build(r, c, h),
      build(r, c + h, h),
      build(r + h, c, h),
      build(r + h, c + h, h),
    );
  }
  return build(0, 0, grid.length);
}
