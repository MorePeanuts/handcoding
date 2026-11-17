function longestPalindrome(s: string): string {
  let start = 0,
    length = 0;
  for (let i = 0; i < s.length; i++)
    for (let parity = 0; parity < 2; parity++) {
      let l = i,
        r = i + parity;
      while (l >= 0 && r < s.length && s[l] === s[r]) {
        l--;
        r++;
      }
      if (r - l - 1 > length) {
        start = l + 1;
        length = r - l - 1;
      }
    }
  return s.slice(start, start + length);
}
