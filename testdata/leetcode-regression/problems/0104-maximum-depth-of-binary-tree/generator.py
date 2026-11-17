def skewed(rng, n, side):
    """A path of n nodes, each the left or the right child of the last: the
    deepest tree of n nodes, which plain recursion goes n calls deep into."""
    root = [1]
    for v in range(2, n + 1):
        root += [v, None] if side == "left" else [None, v]
    return {"root": root}
