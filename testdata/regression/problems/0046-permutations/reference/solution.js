"use strict";
function permute(nums) {
  const out = [];
  const path = [];
  const used = new Array(nums.length).fill(false);
  function visit() {
    if (path.length === nums.length) {
      out.push(path.slice());
      return;
    }
    for (let i = 0; i < nums.length; i++)
      if (!used[i]) {
        used[i] = true;
        path.push(nums[i]);
        visit();
        path.pop();
        used[i] = false;
      }
  }
  visit();
  return out;
}
