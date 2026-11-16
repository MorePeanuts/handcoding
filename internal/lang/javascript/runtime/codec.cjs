"use strict";
const types = require("./types.cjs");
const MAX = 1000000;
function check(ok, message) {
  if (!ok) throw new Error(message);
}
class Numeric {
  constructor(text) {
    this.text = text;
  }
}
function parse(text) {
  return JSON.parse(text, (key, value, context) => {
    if (typeof value !== "number") return value;
    check(
      context && typeof context.source === "string",
      "Node.js lacks lossless JSON parsing",
    );
    return new Numeric(context.source);
  });
}
function scalar(t, v) {
  if (v instanceof Numeric) {
    const n = Number(v.text);
    if (t === "integer" || t === "long")
      check(
        Number.isSafeInteger(n) && BigInt(n) === BigInt(v.text),
        `${t} input is outside JavaScript safe integer range: ${v.text}`,
      );
    v = n;
  }
  if (t === "integer" || t === "long") {
    check(
      typeof v === "number" && Number.isSafeInteger(v),
      `${t} requires a safe integer number`,
    );
    if (t === "integer")
      check(
        v >= -2147483648 && v <= 2147483647,
        "integer outside signed 32-bit range",
      );
    return v;
  }
  if (t === "double") {
    check(
      typeof v === "number" && Number.isFinite(v),
      "double requires a finite number",
    );
    return v;
  }
  if (t === "boolean") {
    check(typeof v === "boolean", "expected boolean");
    return v;
  }
  if (t === "string" || t === "character") {
    check(typeof v === "string", "expected string");
    if (t === "character")
      check(v.length === 1 && v.charCodeAt(0) < 128, "character must be ASCII");
    return v;
  }
  if (t === "void") return null;
  throw new Error("unknown scalar " + t);
}
const scalars = new Set([
  "integer",
  "long",
  "double",
  "boolean",
  "string",
  "character",
  "void",
]);
const quadLinks = ["topLeft", "topRight", "bottomLeft", "bottomRight"];
function decode(t, v) {
  if (t.endsWith("[]")) {
    check(Array.isArray(v), "expected array");
    return v.map((x) => decode(t.slice(0, -2), x));
  }
  if (scalars.has(t)) return scalar(t, v);
  if (t === "NestedInteger") {
    if (v instanceof Numeric)
      return new types.NestedInteger(scalar("integer", v));
    check(Array.isArray(v), "expected nested integer");
    const n = new types.NestedInteger();
    for (const x of v) n.add(decode(t, x));
    return n;
  }
  if (t === "Employee")
    return new types.Employee(
      scalar("integer", v[0]),
      scalar("integer", v[1]),
      decode("integer[]", v[2]),
    );
  check(Array.isArray(v) || v === null, "expected serialized node array");
  if (!v || !v.length || v[0] === null) return null;
  const node = (x) => new types[t](scalar("integer", x));
  if (t === "ListNode") {
    const head = new types.ListNode();
    let tail = head;
    for (const x of v) {
      tail.next = node(x);
      tail = tail.next;
    }
    return head.next;
  }
  if (t === "TreeNode" || t === "NextTreeNode" || t === "QuadNode") {
    const make =
      t === "QuadNode"
        ? (x) =>
            new types.QuadNode(
              !!scalar("integer", x[1]),
              !!scalar("integer", x[0]),
            )
        : node;
    const root = make(v[0]),
      q = [root];
    let k = 1;
    for (let i = 0; k < v.length; i++) {
      check(i < q.length, "invalid tree");
      for (const key of t === "QuadNode" ? quadLinks : ["left", "right"]) {
        if (k < v.length && v[k] !== null) {
          q[i][key] = make(v[k]);
          q.push(q[i][key]);
        }
        k++;
      }
    }
    return root;
  }
  if (t === "GraphNode") {
    const ns = v.map((_, i) => new types.GraphNode(i + 1));
    v.forEach((row, i) => {
      ns[i].neighbors = row.map((x) => ns[scalar("integer", x) - 1]);
    });
    return ns[0];
  }
  if (t === "NaryNode") {
    const root = node(v[0]),
      q = [root];
    let k = 2;
    for (let i = 0; k < v.length; i++) {
      check(i < q.length, "invalid N-ary tree");
      while (k < v.length && v[k] !== null) {
        const n = node(v[k++]);
        q[i].children.push(n);
        q.push(n);
      }
      k++;
    }
    return root;
  }
  if (t === "RandomListNode") {
    const ns = v.map((x) => node(x[0]));
    v.forEach((row, i) => {
      ns[i].next = ns[i + 1] || null;
      ns[i].random = row[1] === null ? null : ns[scalar("integer", row[1])];
    });
    return ns[0];
  }
  if (t === "MultilevelNode") {
    let root = null,
      above = [],
      i = 0;
    while (i < v.length) {
      let parent = null;
      if (above.length) {
        i++;
        let k = 0;
        while (i < v.length && v[i] === null) {
          i++;
          k++;
        }
        if (i === v.length) break;
        parent = above[k];
        check(parent, "invalid multilevel parent");
      }
      const level = [];
      while (i < v.length && v[i] !== null) {
        const n = node(v[i++]);
        if (level.length) {
          n.prev = level.at(-1);
          n.prev.next = n;
        }
        level.push(n);
      }
      check(level.length, "invalid multilevel list");
      if (parent) parent.child = level[0];
      else root = level[0];
      above = level;
    }
    return root;
  }
  throw new Error("unknown node " + t);
}
function trim(a) {
  while (a.length && a.at(-1) === null) a.pop();
  return a;
}
function encode(t, v) {
  if (t.endsWith("[]")) {
    if (v === null) return [];
    check(Array.isArray(v), "expected array result");
    return v.map((x) => encode(t.slice(0, -2), x));
  }
  if (scalars.has(t)) return scalar(t, v);
  if (t === "NestedInteger") {
    check(v && typeof v.isInteger === "function", "expected NestedInteger");
    return v.isInteger()
      ? scalar("integer", v.getInteger())
      : v.getList().map((x) => encode(t, x));
  }
  if (t === "Employee") {
    check(v, "expected Employee");
    return [
      scalar("integer", v.id),
      scalar("integer", v.importance),
      encode("integer[]", v.subordinates),
    ];
  }
  check(v !== undefined, "undefined node result");
  if (v === null) return [];
  const seen = new Set(),
    a = [];
  const visit = (n) => {
    check(
      n && typeof n === "object" && !seen.has(n) && seen.size < MAX,
      "node cycle, shared node or oversized structure",
    );
    seen.add(n);
  };
  const value = (n) => scalar("integer", n.val);
  if (t === "ListNode") {
    for (let n = v; n !== null; n = n.next) {
      visit(n);
      a.push(value(n));
    }
    return a;
  }
  if (t === "TreeNode" || t === "QuadNode") {
    const q = [v];
    for (let i = 0; i < q.length; i++) {
      const n = q[i];
      if (n === null) {
        a.push(null);
        continue;
      }
      visit(n);
      if (t === "TreeNode") {
        a.push(value(n));
        q.push(n.left, n.right);
      } else {
        check(
          typeof n.isLeaf === "boolean" && typeof n.val === "boolean",
          "invalid QuadNode",
        );
        a.push([n.isLeaf ? 1 : 0, n.isLeaf ? (n.val ? 1 : 0) : 1]);
        for (const k of quadLinks) q.push(n[k]);
      }
    }
    return trim(a);
  }
  if (t === "GraphNode") {
    const q = [v],
      byValue = new Map();
    seen.add(v);
    for (let i = 0; i < q.length; i++) {
      const n = q[i],
        val = value(n);
      check(
        q.length <= MAX && val > 0 && !byValue.has(val),
        "invalid graph node values",
      );
      byValue.set(val, n);
      for (const c of n.neighbors) {
        check(c, "null graph neighbor");
        if (!seen.has(c)) {
          seen.add(c);
          q.push(c);
        }
      }
    }
    check(v.val === 1, "graph root must be node 1");
    for (let i = 1; i <= q.length; i++) {
      check(byValue.has(i), "graph values must be 1 to n");
      a.push(byValue.get(i).neighbors.map(value));
    }
    return a;
  }
  if (t === "NaryNode") {
    visit(v);
    a.push(value(v), null);
    const q = [v];
    for (let i = 0; i < q.length; i++) {
      for (const c of q[i].children) {
        visit(c);
        a.push(value(c));
        q.push(c);
      }
      a.push(null);
    }
    return trim(a);
  }
  if (t === "RandomListNode") {
    const ns = [],
      index = new Map();
    for (let n = v; n !== null; n = n.next) {
      visit(n);
      index.set(n, ns.length);
      ns.push(n);
    }
    return ns.map((n) => {
      check(
        n.random === null || index.has(n.random),
        "random pointer outside list",
      );
      return [value(n), n.random === null ? null : index.get(n.random)];
    });
  }
  if (t === "NextTreeNode") {
    let level = [v];
    while (level.length) {
      const next = [],
        row = [];
      level.forEach((n, i) => {
        visit(n);
        check(n.next === (level[i + 1] || null), "incorrect next pointer");
        row.push(value(n));
        for (const k of ["left", "right"]) if (n[k] !== null) next.push(n[k]);
      });
      a.push(row);
      level = next;
    }
    return a;
  }
  if (t === "MultilevelNode") {
    let head = v;
    while (head !== null) {
      let child = null,
        at = 0,
        i = 0,
        prev = null;
      for (let n = head; n !== null; prev = n, n = n.next, i++) {
        visit(n);
        check(n.prev === prev, "incorrect prev pointer");
        a.push(value(n));
        if (n.child !== null) {
          check(child === null, "multiple child levels cannot be encoded");
          child = n.child;
          at = i;
        }
      }
      if (child !== null) for (let k = 0; k <= at; k++) a.push(null);
      head = child;
    }
    return a;
  }
  throw new Error("unknown node " + t);
}
function links(t, n) {
  if (t === "GraphNode") return n.neighbors;
  if (t === "NaryNode") return n.children;
  const keys = {
    ListNode: ["next"],
    TreeNode: ["left", "right"],
    RandomListNode: ["next", "random"],
    NextTreeNode: ["left", "right", "next"],
    QuadNode: quadLinks,
    MultilevelNode: ["prev", "next", "child"],
  }[t];
  return keys.map((k) => n[k]);
}
function nodes(t, root, unique = true) {
  const q = root === null ? [] : [root],
    seen = new Set();
  for (let i = 0; i < q.length; i++) {
    const n = q[i];
    check(n && q.length <= MAX, "invalid or oversized structure");
    if (seen.has(n)) {
      check(!unique, "input cycle or shared node");
      continue;
    }
    seen.add(n);
    for (const c of links(t, n)) if (c !== null) q.push(c);
  }
  return unique ? q : [...seen];
}
module.exports = { parse, scalar, decode, encode, nodes, check };
