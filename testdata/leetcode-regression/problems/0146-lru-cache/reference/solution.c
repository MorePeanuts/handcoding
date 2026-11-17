typedef struct Entry {
    int key, value;
    struct Entry *prev, *next;
    UT_hash_handle hh;
} Entry;
typedef struct {
    int capacity;
    Entry *entries, order;
} LRUCache;
static void unlinkEntry(Entry* e) {
    e->prev->next = e->next;
    e->next->prev = e->prev;
}
static void promote(LRUCache* obj, Entry* e) {
    e->next = obj->order.next;
    e->prev = &obj->order;
    e->next->prev = e;
    obj->order.next = e;
}
LRUCache* lRUCacheCreate(int capacity) {
    LRUCache* obj = calloc(1, sizeof(*obj));
    obj->capacity = capacity;
    obj->order.next = obj->order.prev = &obj->order;
    return obj;
}
int lRUCacheGet(LRUCache* obj, int key) {
    Entry* e;
    HASH_FIND_INT(obj->entries, &key, e);
    if (!e) return -1;
    unlinkEntry(e);
    promote(obj, e);
    return e->value;
}
void lRUCachePut(LRUCache* obj, int key, int value) {
    Entry* e;
    HASH_FIND_INT(obj->entries, &key, e);
    if (e) {
        e->value = value;
        unlinkEntry(e);
        promote(obj, e);
        return;
    }
    e = calloc(1, sizeof(*e));
    e->key = key;
    e->value = value;
    HASH_ADD_INT(obj->entries, key, e);
    promote(obj, e);
    if (HASH_COUNT(obj->entries) > (unsigned)obj->capacity) {
        e = obj->order.prev;
        unlinkEntry(e);
        HASH_DEL(obj->entries, e);
        free(e);
    }
}
void lRUCacheFree(LRUCache* obj) {
    Entry *e, *tmp;
    HASH_ITER(hh, obj->entries, e, tmp) {
        HASH_DEL(obj->entries, e);
        free(e);
    }
    free(obj);
}
