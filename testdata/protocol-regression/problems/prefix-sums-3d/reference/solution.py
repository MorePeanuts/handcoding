class Solution:
    def prefixSums(self, grid: list[list[list[int]]]) -> list[list[list[int]]]:
        # Accumulate along each axis in turn.
        s = [[row[:] for row in plane] for plane in grid]
        a, b, c = len(s), len(s[0]), len(s[0][0])
        for i in range(a):
            for j in range(b):
                for k in range(1, c):
                    s[i][j][k] += s[i][j][k - 1]
        for i in range(a):
            for j in range(1, b):
                for k in range(c):
                    s[i][j][k] += s[i][j - 1][k]
        for i in range(1, a):
            for j in range(b):
                for k in range(c):
                    s[i][j][k] += s[i - 1][j][k]
        return s
