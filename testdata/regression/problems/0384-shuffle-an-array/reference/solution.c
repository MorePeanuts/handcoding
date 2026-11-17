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
    int *original, *output, size;
    uint64_t state;
} Solution;
Solution* solutionCreate(int* nums, int numsSize) {
    Solution* obj = malloc(sizeof(*obj));
    obj->size = numsSize;
    obj->state = 1;
    obj->original = malloc((size_t)numsSize * sizeof(int));
    obj->output = malloc((size_t)numsSize * sizeof(int));
    if (numsSize) memcpy(obj->original, nums, (size_t)numsSize * sizeof(int));
    return obj;
}
int* solutionReset(Solution* obj, int* retSize) {
    *retSize = obj->size;
    if (obj->size)
        memcpy(obj->output, obj->original, (size_t)obj->size * sizeof(int));
    return obj->output;
}
int* solutionShuffle(Solution* obj, int* retSize) {
    int* out = solutionReset(obj, retSize);
    for (int i = obj->size - 1; i > 0; --i) {
        int j = (int)bounded(&obj->state, (uint64_t)i + 1), tmp = out[i];
        out[i] = out[j];
        out[j] = tmp;
    }
    return out;
}
void solutionFree(Solution* obj) {
    free(obj->original);
    free(obj->output);
    free(obj);
}
