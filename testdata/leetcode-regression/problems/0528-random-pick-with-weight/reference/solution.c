/* SplitMix64 with a fixed seed makes statistical regression failures
 * reproducible. Rejection sampling avoids modulo bias for arbitrary bounds. */
static uint64_t bounded(uint64_t* state, uint64_t bound) {
    uint64_t value, threshold = (uint64_t)(-bound) % bound;
    do {
        value = (*state += UINT64_C(0x9e3779b97f4a7c15));
        value = (value ^ (value >> 30)) * UINT64_C(0xbf58476d1ce4e5b9);
        value = (value ^ (value >> 27)) * UINT64_C(0x94d049bb133111eb);
        value ^= value >> 31;
    } while (value < threshold);
    return value % bound;
}

typedef struct {
    uint64_t* prefix;
    int size;
    uint64_t state;
} Solution;
Solution* solutionCreate(int* w, int wSize) {
    Solution* obj = malloc(sizeof(*obj));
    obj->size = wSize;
    obj->state = 1;
    obj->prefix = malloc((size_t)wSize * sizeof(*obj->prefix));
    uint64_t total = 0;
    for (int i = 0; i < wSize; ++i) obj->prefix[i] = (total += (uint64_t)w[i]);
    return obj;
}
int solutionPickIndex(Solution* obj) {
    uint64_t target = bounded(&obj->state, obj->prefix[obj->size - 1]);
    int lo = 0, hi = obj->size - 1;
    while (lo < hi) {
        int mid = lo + (hi - lo) / 2;
        if (obj->prefix[mid] > target)
            hi = mid;
        else
            lo = mid + 1;
    }
    return lo;
}
void solutionFree(Solution* obj) {
    free(obj->prefix);
    free(obj);
}
