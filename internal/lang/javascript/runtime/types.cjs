"use strict";
class ListNode {
  constructor(val = 0, next = null) {
    this.val = val;
    this.next = next;
  }
}
class TreeNode {
  constructor(val = 0, left = null, right = null) {
    this.val = val;
    this.left = left;
    this.right = right;
  }
}
class GraphNode {
  constructor(val = 0, neighbors = []) {
    this.val = val;
    this.neighbors = neighbors;
  }
}
class NaryNode {
  constructor(val = 0, children = []) {
    this.val = val;
    this.children = children;
  }
}
class RandomListNode {
  constructor(val = 0, next = null, random = null) {
    Object.assign(this, { val, next, random });
  }
}
class NextTreeNode {
  constructor(val = 0, left = null, right = null, next = null) {
    Object.assign(this, { val, left, right, next });
  }
}
class QuadNode {
  constructor(
    val = false,
    isLeaf = false,
    topLeft = null,
    topRight = null,
    bottomLeft = null,
    bottomRight = null,
  ) {
    Object.assign(this, {
      val,
      isLeaf,
      topLeft,
      topRight,
      bottomLeft,
      bottomRight,
    });
  }
}
class MultilevelNode {
  constructor(val = 0, prev = null, next = null, child = null) {
    Object.assign(this, { val, prev, next, child });
  }
}
class NestedInteger {
  constructor(value) {
    this.value = value;
    this.items = [];
  }
  isInteger() {
    return this.value !== undefined;
  }
  getInteger() {
    return this.isInteger() ? this.value : null;
  }
  setInteger(value) {
    this.value = value;
    this.items = [];
  }
  add(item) {
    this.value = undefined;
    this.items.push(item);
  }
  getList() {
    return this.isInteger() ? [] : this.items;
  }
}
class Employee {
  constructor(id = 0, importance = 0, subordinates = []) {
    Object.assign(this, { id, importance, subordinates });
  }
}
module.exports = {
  ListNode,
  TreeNode,
  GraphNode,
  NaryNode,
  RandomListNode,
  NextTreeNode,
  QuadNode,
  MultilevelNode,
  NestedInteger,
  Employee,
};
