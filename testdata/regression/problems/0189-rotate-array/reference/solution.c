static void reverse(int* a, int lo, int hi) {
    while (lo < hi) {
        int tmp = a[lo];
        a[lo++] = a[hi];
        a[hi--] = tmp;
    }
}
void rotate(int* nums, int numsSize, int k) {
    if (!numsSize) return;
    k %= numsSize;
    reverse(nums, 0, numsSize - 1);
    reverse(nums, 0, k - 1);
    reverse(nums, k, numsSize - 1);
}
