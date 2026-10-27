# Any rearrangement of nums with nums[0] < nums[1] > nums[2] < ... is
# accepted. input holds nums before the call, output after it.

from collections import Counter


def check(input, output, expected):
    nums = input["nums"]
    if not isinstance(output, list):
        return "nums must stay an array, got %r" % (output,)
    if len(output) != len(nums):
        return "nums has %d elements after the call, but %d before" % (len(output), len(nums))
    before, after = Counter(nums), Counter(output)
    if before != after:
        v = next(v for v in before.keys() | after.keys() if before[v] != after[v])
        return "not a rearrangement of nums: %r occurs %d times, but %d times in the input" % (v, after[v], before[v])
    for i in range(len(output) - 1):
        a, b = output[i], output[i + 1]
        if i % 2 == 0 and not a < b:
            return "nums[%d] = %r is not less than nums[%d] = %r" % (i, a, i + 1, b)
        if i % 2 == 1 and not a > b:
            return "nums[%d] = %r is not greater than nums[%d] = %r" % (i, a, i + 1, b)
