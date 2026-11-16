"use strict";
function getImportance(employees, id) {
  const byId = new Map(employees.map((e) => [e.id, e]));
  const stack = [id];
  let total = 0;
  while (stack.length) {
    const e = byId.get(stack.pop());
    total += e.importance;
    stack.push(...e.subordinates);
  }
  return total;
}
