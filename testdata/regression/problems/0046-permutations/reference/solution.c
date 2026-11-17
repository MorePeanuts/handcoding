static void generate(int* nums, int n, int at, int** out, int* count) {
    if (at == n) {
        out[*count] = malloc((size_t)n * sizeof(int));
        if (n) memcpy(out[*count], nums, (size_t)n * sizeof(int));
        ++*count;
        return;
    }
    for (int i = at; i < n; ++i) {
        int tmp = nums[at];
        nums[at] = nums[i];
        nums[i] = tmp;
        generate(nums, n, at + 1, out, count);
        tmp = nums[at];
        nums[at] = nums[i];
        nums[i] = tmp;
    }
}
int** permute(int* nums, int numsSize, int* returnSize,
              int** returnColumnSizes) {
    int count = 1;
    for (int i = 2; i <= numsSize; ++i) count *= i;
    int** out = malloc((size_t)count * sizeof(*out));
    *returnColumnSizes = malloc((size_t)count * sizeof(int));
    for (int i = 0; i < count; ++i) (*returnColumnSizes)[i] = numsSize;
    *returnSize = 0;
    generate(nums, numsSize, 0, out, returnSize);
    return out;
}
