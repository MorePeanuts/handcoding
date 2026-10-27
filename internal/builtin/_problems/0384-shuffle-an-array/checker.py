# reset must return the original nums and every shuffle a permutation of it.
# The shuffles of a case are then tested for uniformity: how often each value
# lands at each position, and for up to 6 elements how often each
# permutation occurs. Each count is binomial, Bin(S, p) for S shuffles, and
# is rejected when the Chernoff bound on a deviation at least as large,
# exp(-S * KL(count / S || p)), is below ALPHA / (2 * buckets). By the union
# bound over all buckets and both tails, a uniform shuffle fails a case with
# probability below ALPHA = 1e-9. The identity shuffle, or the same
# permutation every time, fails as soon as a case has a few dozen shuffles.

import math
from itertools import permutations

ALPHA = 1e-9


def _kl(q, p):
    """KL divergence between Bernoulli(q) and Bernoulli(p)."""
    d = 0.0
    if q > 0:
        d += q * math.log(q / p)
    if q < 1:
        d += (1 - q) * math.log((1 - q) / (1 - p))
    return d


def _implausible(count, total, p, buckets):
    """Whether count successes in total Bin(total, p) trials are implausible."""
    if total == 0 or count == total * p:
        return False
    return total * _kl(count / total, p) > math.log(2 * buckets / ALPHA)


def check(input, output, expected):
    ops, nums = input["ops"], input["args"][0][0]
    n = len(nums)
    if not isinstance(output, list) or len(output) != len(ops):
        return "expected one result per call (%d), got %r" % (len(ops), output)
    want = sorted(nums)
    shuffles = []
    for i in range(1, len(ops)):
        got = output[i]
        if ops[i] == "reset":
            if got != nums:
                return "call %d (reset) returned %r, not the original nums %r" % (i, got, nums)
        elif not isinstance(got, list) or sorted(got) != want:
            return "call %d (shuffle) returned %r, which is not a permutation of nums %r" % (i, got, nums)
        else:
            shuffles.append(got)
    s = len(shuffles)
    if s == 0 or n < 2:
        return None

    perms = math.factorial(n) if n <= 6 else 0
    buckets = n * n + perms
    for pos in range(n):
        counts = dict.fromkeys(nums, 0)
        for sh in shuffles:
            counts[sh[pos]] += 1
        for v, c in counts.items():
            if _implausible(c, s, 1 / n, buckets):
                return "%d of %d shuffles put %r at index %d; a uniform shuffle does that about %.0f times" % (
                    c, s, v, pos, s / n)
    if perms:
        counts = dict.fromkeys(permutations(nums), 0)
        for sh in shuffles:
            counts[tuple(sh)] += 1
        for perm, c in counts.items():
            if _implausible(c, s, 1 / perms, buckets):
                return "%d of %d shuffles returned %r; a uniform shuffle does that about %.0f times" % (
                    c, s, list(perm), s / perms)
    return None
