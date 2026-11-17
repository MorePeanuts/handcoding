# Every pick must be an index of w, and index i must be picked about
# S * w[i] / sum(w) times in S picks. Each count is binomial, Bin(S, p), and
# is rejected when the Chernoff bound on a deviation at least as large,
# exp(-S * KL(count / S || p)), is below ALPHA / (2 * len(w)). By the union
# bound over all indices and both tails, a correct solution fails a case with
# probability below ALPHA = 1e-9, while ignoring the weights or always
# picking the heaviest index fails the cases with a few thousand picks.

import math

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
    ops, w = input["ops"], input["args"][0][0]
    if not isinstance(output, list) or len(output) != len(ops):
        return "expected one result per call (%d), got %r" % (len(ops), output)
    counts = [0] * len(w)
    for i in range(1, len(ops)):
        pick = output[i]
        if not isinstance(pick, int) or isinstance(pick, bool) or not 0 <= pick < len(w):
            return "call %d returned %r, which is not an index of w (0 to %d)" % (i, pick, len(w) - 1)
        counts[pick] += 1
    s, total = len(ops) - 1, sum(w)
    for i, c in enumerate(counts):
        if _implausible(c, s, w[i] / total, len(w)):
            return "index %d (weight %d of %d) was picked %d times in %d picks; expected about %.0f" % (
                i, w[i], total, c, s, s * w[i] / total)
    return None
