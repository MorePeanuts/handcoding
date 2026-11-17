from bisect import bisect_right


class Solution:
    def nextGreatestLetter(self, letters: list[str], target: str) -> str:
        i = bisect_right(letters, target)
        return letters[i % len(letters)]
