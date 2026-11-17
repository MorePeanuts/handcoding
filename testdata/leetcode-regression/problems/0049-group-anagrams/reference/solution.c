typedef struct Group {
    int counts[26], index, size, capacity;
    char** words;
    UT_hash_handle hh;
} Group;
char*** groupAnagrams(char** strs, int strsSize, int* returnSize,
                      int** returnColumnSizes) {
    Group *groups = NULL, *g, *tmp;
    char*** out = malloc((size_t)strsSize * sizeof(*out));
    *returnColumnSizes = malloc((size_t)strsSize * sizeof(int));
    *returnSize = 0;
    for (int i = 0; i < strsSize; ++i) {
        int key[26] = {0};
        for (const char* c = strs[i]; *c; ++c) ++key[*c - 'a'];
        HASH_FIND(hh, groups, key, sizeof(key), g);
        if (!g) {
            g = calloc(1, sizeof(*g));
            memcpy(g->counts, key, sizeof(key));
            g->index = (*returnSize)++;
            g->capacity = 4;
            g->words = malloc((size_t)g->capacity * sizeof(*g->words));
            HASH_ADD(hh, groups, counts, sizeof(key), g);
        }
        if (g->size == g->capacity) {
            g->capacity *= 2;
            g->words =
                realloc(g->words, (size_t)g->capacity * sizeof(*g->words));
        }
        g->words[g->size++] = strs[i];
    }
    HASH_ITER(hh, groups, g, tmp) {
        out[g->index] = g->words;
        (*returnColumnSizes)[g->index] = g->size;
        HASH_DEL(groups, g);
        free(g);
    }
    return out;
}
