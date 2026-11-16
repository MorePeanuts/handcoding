"use strict";
class LRUCache {
  capacity;
  data = new Map();
  constructor(capacity) {
    this.capacity = capacity;
  }
  get(key) {
    if (!this.data.has(key)) return -1;
    const value = this.data.get(key);
    this.data.delete(key);
    this.data.set(key, value);
    return value;
  }
  put(key, value) {
    this.data.delete(key);
    this.data.set(key, value);
    if (this.data.size > this.capacity)
      this.data.delete(this.data.keys().next().value);
  }
}
