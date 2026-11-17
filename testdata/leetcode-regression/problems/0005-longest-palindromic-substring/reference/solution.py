class Solution:
    def longestPalindrome(self, s: str) -> str:
        # Expand around every center; keeps the leftmost longest palindrome.
        best = s[0]
        for center in range(2 * len(s) - 1):
            lo, hi = center // 2, (center + 1) // 2
            while lo >= 0 and hi < len(s) and s[lo] == s[hi]:
                lo -= 1
                hi += 1
            if hi - lo - 1 > len(best):
                best = s[lo + 1:hi]
        return best
