double findMedianSortedArrays(int* a, int aSize, int* b, int bSize) {
    int i = 0, j = 0, prev = 0, curr = 0, n = aSize + bSize;
    for (int k = 0; k <= n / 2; ++k) {
        prev = curr;
        curr = j == bSize || (i < aSize && a[i] <= b[j]) ? a[i++] : b[j++];
    }
    return n % 2 ? curr : ((double)prev + curr) / 2;
}
