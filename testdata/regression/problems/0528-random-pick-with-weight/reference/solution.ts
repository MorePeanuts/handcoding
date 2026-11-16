class Solution {
  private prefix: number[] = [];
  private total = 0;
  constructor(w: number[]) {
    for (const x of w) {
      this.total += x;
      this.prefix.push(this.total);
    }
  }
  pickIndex(): number {
    const x = Math.random() * this.total;
    let lo = 0,
      hi = this.prefix.length - 1;
    while (lo < hi) {
      const m = (lo + hi) >> 1;
      if (this.prefix[m] <= x) lo = m + 1;
      else hi = m;
    }
    return lo;
  }
}
