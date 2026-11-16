function makeIntegerBeautiful(n: number, target: number): number {
  function sum(x: number) {
    let s = 0;
    while (x) {
      s += x % 10;
      x = Math.floor(x / 10);
    }
    return s;
  }
  let value = n,
    place = 1;
  while (sum(value) > target) {
    const digit = Math.floor(value / place) % 10;
    value += (10 - digit) * place;
    place *= 10;
  }
  return value - n;
}
