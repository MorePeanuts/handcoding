"use strict";
function findMedianSortedArrays(nums1, nums2) {
  if (nums1.length > nums2.length) return findMedianSortedArrays(nums2, nums1);
  const m = nums1.length,
    n = nums2.length;
  let lo = 0,
    hi = m;
  while (lo <= hi) {
    const i = (lo + hi) >> 1,
      j = ((m + n + 1) >> 1) - i;
    const a = i ? nums1[i - 1] : -Infinity,
      b = i < m ? nums1[i] : Infinity;
    const c = j ? nums2[j - 1] : -Infinity,
      d = j < n ? nums2[j] : Infinity;
    if (a > d) hi = i - 1;
    else if (c > b) lo = i + 1;
    else
      return (m + n) % 2
        ? Math.max(a, c)
        : (Math.max(a, c) + Math.min(b, d)) / 2;
  }
  return 0;
}
