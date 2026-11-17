struct NestedInteger* deserialize(char* s) {
    if (*s != '[') {
        struct NestedInteger* n = NestedIntegerInit();
        NestedIntegerSetInteger(n, (int)strtol(s, NULL, 10));
        return n;
    }
    int size = 0, capacity = 16;
    struct NestedInteger** stack = malloc((size_t)capacity * sizeof(*stack));
    struct NestedInteger* root = NULL;
    for (char* p = s; *p;) {
        if (*p == '[') {
            struct NestedInteger* n = NestedIntegerInit();
            if (size)
                NestedIntegerAdd(stack[size - 1], n);
            else
                root = n;
            if (size == capacity) {
                capacity *= 2;
                stack = realloc(stack, (size_t)capacity * sizeof(*stack));
            }
            stack[size++] = n;
            ++p;
        } else if (*p == ']') {
            --size;
            ++p;
        } else if (*p == ',')
            ++p;
        else {
            struct NestedInteger* n = NestedIntegerInit();
            NestedIntegerSetInteger(n, (int)strtol(p, &p, 10));
            NestedIntegerAdd(stack[size - 1], n);
        }
    }
    free(stack);
    return root;
}
