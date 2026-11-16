class Solution {
  private original: number[];
  constructor(nums: number[]) {
    this.original = nums.slice();
  }
  reset(): number[] {
    return this.original.slice();
  }
  shuffle(): number[] {
    const a = this.original.slice();
    for (let i = a.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
  }
}
