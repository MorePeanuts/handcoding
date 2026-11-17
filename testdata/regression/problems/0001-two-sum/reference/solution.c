typedef struct Entry {
    int value, index;
    UT_hash_handle hh;
} Entry;
int* twoSum(int* nums, int numsSize, int target, int* returnSize) {
    Entry *seen = NULL, *entry, *tmp;
    int* result = malloc(2 * sizeof(*result));
    *returnSize = 0;
    for (int i = 0; i < numsSize; ++i) {
        long long other = (long long)target - nums[i];
        entry = NULL;
        if (other >= INT_MIN && other <= INT_MAX) {
            int key = (int)other;
            HASH_FIND_INT(seen, &key, entry);
        }
        if (entry) {
            result[0] = entry->index;
            result[1] = i;
            *returnSize = 2;
            break;
        }
        HASH_FIND_INT(seen, &nums[i], entry);
        if (!entry) {
            entry = malloc(sizeof(*entry));
            entry->value = nums[i];
            entry->index = i;
            HASH_ADD_INT(seen, value, entry);
        }
    }
    HASH_ITER(hh, seen, entry, tmp) {
        HASH_DEL(seen, entry);
        free(entry);
    }
    return result;
}
