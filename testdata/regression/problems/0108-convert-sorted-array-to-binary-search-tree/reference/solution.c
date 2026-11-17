static struct TreeNode* build(int* nums, int lo, int hi) {
    if (lo >= hi) return NULL;
    int mid = lo + (hi - lo) / 2;
    struct TreeNode* n = malloc(sizeof(*n));
    *n = (struct TreeNode){nums[mid], build(nums, lo, mid),
                           build(nums, mid + 1, hi)};
    return n;
}
struct TreeNode* sortedArrayToBST(int* nums, int numsSize) {
    return build(nums, 0, numsSize);
}
