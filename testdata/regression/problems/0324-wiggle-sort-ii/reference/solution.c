static int compare(const void* a, const void* b) {
    int x = *(const int*)a, y = *(const int*)b;
    return (x > y) - (x < y);
}
void wiggleSort(int* nums, int numsSize) {
    int* sorted = malloc((size_t)numsSize * sizeof(int));
    if (numsSize) memcpy(sorted, nums, (size_t)numsSize * sizeof(int));
    qsort(sorted, (size_t)numsSize, sizeof(int), compare);
    int lo = (numsSize + 1) / 2 - 1, hi = numsSize - 1;
    for (int i = 0; i < numsSize; ++i) nums[i] = sorted[i % 2 ? hi-- : lo--];
    free(sorted);
}
