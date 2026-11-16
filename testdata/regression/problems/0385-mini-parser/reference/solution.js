"use strict";
function deserialize(s) {
  const stack = [];
  let current = null,
    number = "",
    root = null;
  function flush() {
    if (number) {
      const n = new NestedInteger(Number(number));
      if (current) current.add(n);
      else root = n;
      number = "";
    }
  }
  for (const c of s) {
    if (c === "[") {
      const n = new NestedInteger();
      if (current) {
        current.add(n);
        stack.push(current);
      } else root = n;
      current = n;
    } else if (c === "]") {
      flush();
      current = stack.pop() || null;
    } else if (c === ",") flush();
    else number += c;
  }
  flush();
  return root;
}
