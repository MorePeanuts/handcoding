function nextGreatestLetter(letters: string[], target: string): string {
  let lo = 0,
    hi = letters.length;
  while (lo < hi) {
    const m = (lo + hi) >> 1;
    if (letters[m] <= target) lo = m + 1;
    else hi = m;
  }
  return letters[lo % letters.length];
}
