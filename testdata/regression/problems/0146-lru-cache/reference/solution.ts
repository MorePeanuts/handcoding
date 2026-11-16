class LRUCache {
  private capacity: number;
  private data = new Map<number, number>();
  constructor(capacity: number) {
    this.capacity = capacity;
  }
  get(key: number): number {
    if (!this.data.has(key)) return -1;
    const value = this.data.get(key);
    this.data.delete(key);
    this.data.set(key, value);
    return value;
  }
  put(key: number, value: number): void {
    this.data.delete(key);
    this.data.set(key, value);
    if (this.data.size > this.capacity)
      this.data.delete(this.data.keys().next().value);
  }
}
