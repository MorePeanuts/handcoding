class Solution:
    def makeIntegerBeautiful(self, n: int, target: int) -> int:
        # Round n up to the next multiple of 10, 100, ... until its digit sum
        # is small enough: each rounding zeroes one more low digit.
        def digit_sum(v: int) -> int:
            return sum(int(d) for d in str(v))

        m, unit = n, 1
        while digit_sum(m) > target:
            unit *= 10
            m = (n + unit - 1) // unit * unit
        return m - n
