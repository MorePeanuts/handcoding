int findPeakElement(int* nums, int numsSize) {
    int lo = 0, hi = numsSize - 1;
    while (lo < hi) {
        int mid = lo + (hi - lo) / 2;
        if (nums[mid] > nums[mid + 1])
            hi = mid;
        else
            lo = mid + 1;
    }
    return lo;
}
