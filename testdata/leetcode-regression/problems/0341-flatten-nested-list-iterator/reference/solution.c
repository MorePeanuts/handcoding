typedef struct {
    struct NestedInteger** list;
    int size, at;
} Frame;
struct NestedIterator {
    Frame* stack;
    int size, capacity;
};
struct NestedIterator* nestedIterCreate(struct NestedInteger** nestedList,
                                        int nestedListSize) {
    struct NestedIterator* obj = malloc(sizeof(*obj));
    obj->capacity = 16;
    obj->size = 1;
    obj->stack = malloc((size_t)obj->capacity * sizeof(*obj->stack));
    obj->stack[0] = (Frame){nestedList, nestedListSize, 0};
    return obj;
}
bool nestedIterHasNext(struct NestedIterator* obj) {
    while (obj->size) {
        Frame* f = &obj->stack[obj->size - 1];
        if (f->at == f->size) {
            --obj->size;
            continue;
        }
        struct NestedInteger* n = f->list[f->at];
        if (NestedIntegerIsInteger(n)) return true;
        ++f->at;
        if (obj->size == obj->capacity) {
            obj->capacity *= 2;
            obj->stack = realloc(obj->stack,
                                 (size_t)obj->capacity * sizeof(*obj->stack));
        }
        obj->stack[obj->size++] =
            (Frame){NestedIntegerGetList(n), NestedIntegerGetListSize(n), 0};
    }
    return false;
}
int nestedIterNext(struct NestedIterator* obj) {
    (void)nestedIterHasNext(obj);
    Frame* f = &obj->stack[obj->size - 1];
    return NestedIntegerGetInteger(f->list[f->at++]);
}
void nestedIterFree(struct NestedIterator* obj) {
    free(obj->stack);
    free(obj);
}
