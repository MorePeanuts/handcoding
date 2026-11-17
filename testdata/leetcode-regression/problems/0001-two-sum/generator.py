# Large inputs for Two Sum. Each function returns the input of a case.


def distinct(rng, n):
    """n distinct numbers, the answer at two random places.

    The numbers are even but one, so a pair adds up to the odd target only
    if it holds the odd one: the answer is unique.
    """
    nums = [2 * x for x in rng.sample(range(-(10**8), 10**8), n)]
    i, j = rng.sample(range(n), 2)
    nums[j] += 1
    return {"nums": nums, "target": nums[i] + nums[j]}


def last_pair(rng, n):
    """The answer is the last two numbers, the worst case for a solution
    that checks the pairs in order."""
    nums = list(range(0, 2 * (n - 2), 2)) + [1, 10**8]
    return {"nums": nums, "target": 1 + 10**8}
