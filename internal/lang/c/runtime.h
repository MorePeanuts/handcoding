#ifndef HC_C_RUNTIME_H
#define HC_C_RUNTIME_H
#include <pthread.h>
#include <setjmp.h>
#include <stdatomic.h>
#include <sys/resource.h>
#include <time.h>
#include <yyjson.h>

enum {
    HC_VOID,
    HC_INTEGER,
    HC_LONG,
    HC_DOUBLE,
    HC_BOOLEAN,
    HC_CHARACTER,
    HC_STRING,
    HC_LISTNODE,
    HC_TREENODE,
    HC_NODE,
    HC_NESTEDINTEGER,
    HC_EMPLOYEE
};
typedef struct {
    void* data;
    int size;
    int* rows;
    int** cols;
} hc_value;
static yyjson_mut_doc* hc_doc;
static jmp_buf hc_jump;
static const char* hc_error;
static void hc_fail(const char* s) {
    hc_error = s;
    longjmp(hc_jump, 1);
}
static void hc_check(bool ok, const char* s) {
    if (!ok) hc_fail(s);
}
static void* hc_alloc(size_t n, size_t size) {
    hc_check(!size || n <= SIZE_MAX / size, "allocation size overflow");
    void* p = calloc(n ? n : 1, size ? size : 1);
    hc_check(p != NULL, "out of memory");
    return p;
}
static void* hc_resize(void* p, size_t n, size_t size) {
    hc_check(!size || n <= SIZE_MAX / size, "allocation size overflow");
    p = realloc(p, (n ? n : 1) * size);
    hc_check(p != NULL, "out of memory");
    return p;
}
static int hc_size(size_t n) {
    hc_check(n <= INT_MAX, "array length exceeds C int range");
    return (int)n;
}
static yyjson_mut_val* hc_arr(void) {
    yyjson_mut_val* v = yyjson_mut_arr(hc_doc);
    hc_check(v != NULL, "out of memory");
    return v;
}
static yyjson_mut_val* hc_null(void) { return yyjson_mut_null(hc_doc); }
static yyjson_mut_val* hc_int(long long n) {
    return yyjson_mut_sint(hc_doc, n);
}
static void hc_add(yyjson_mut_val* a, yyjson_mut_val* v) {
    hc_check(v && yyjson_mut_arr_append(a, v), "cannot encode result");
}
static char* hc_string(yyjson_val* v) {
    hc_check(yyjson_is_str(v), "expected string");
    size_t n = yyjson_get_len(v);
    const char* s = yyjson_get_str(v);
    hc_check(!memchr(s, 0, n),
             "C strings cannot contain U+0000; use character[] for zero bytes");
    char* p = hc_alloc(n + 1, 1);
    memcpy(p, s, n);
    return p;
}
struct NestedInteger {
    bool integer;
    int value, size;
    struct NestedInteger** list;
};
struct NestedInteger* NestedIntegerInit(void) {
    return hc_alloc(1, sizeof(struct NestedInteger));
}
bool NestedIntegerIsInteger(struct NestedInteger* n) {
    hc_check(n != NULL, "null NestedInteger");
    return n->integer;
}
int NestedIntegerGetInteger(struct NestedInteger* n) {
    hc_check(n && n->integer, "NestedInteger is not an integer");
    return n->value;
}
void NestedIntegerSetInteger(struct NestedInteger* n, int v) {
    hc_check(n != NULL, "null NestedInteger");
    n->integer = true;
    n->value = v;
    n->size = 0;
}
void NestedIntegerAdd(struct NestedInteger* n, struct NestedInteger* v) {
    hc_check(n && !n->integer && n->size >= 0 && n->size < INT_MAX,
             "NestedInteger is not a list");
    n->list = hc_resize(n->list, (size_t)n->size + 1, sizeof(*n->list));
    n->list[n->size++] = v;
}
struct NestedInteger** NestedIntegerGetList(struct NestedInteger* n) {
    hc_check(n && !n->integer, "NestedInteger is not a list");
    return n->list;
}
int NestedIntegerGetListSize(struct NestedInteger* n) {
    hc_check(n && !n->integer, "NestedInteger is not a list");
    return n->size;
}

typedef struct hc_seen {
    void* ptr;
    size_t index;
    UT_hash_handle hh;
} hc_seen;
typedef struct {
    void** items;
    size_t size, cap;
    hc_seen* seen;
    int kind;
} hc_nodes;
static bool hc_seen_add(hc_nodes* q, void* ptr) {
    hc_seen* item;
    HASH_FIND_PTR(q->seen, &ptr, item);
    if (item) return false;
    item = hc_alloc(1, sizeof(*item));
    item->ptr = ptr;
    item->index = q->size;
    HASH_ADD_PTR(q->seen, ptr, item);
    return true;
}
static void hc_push(hc_nodes* q, void* ptr) {
    hc_check(q->size < 4000001, "node structure too large");
    if (q->size == q->cap) {
        q->cap = q->cap ? q->cap * 2 : 16;
        q->items = hc_resize(q->items, q->cap, sizeof(void*));
    }
    q->items[q->size++] = ptr;
}
static bool hc_visit(hc_nodes* q, void* p) {
    if (!p || !hc_seen_add(q, p)) return false;
    hc_push(q, p);
    return true;
}
static hc_nodes hc_collect(void* root, int kind) {
    hc_nodes q = {.kind = kind};
    hc_visit(&q, root);
    for (size_t i = 0; i < q.size; i++) {
        if (kind == HC_LISTNODE) {
            hc_visit(&q, ((struct ListNode*)q.items[i])->next);
        } else if (kind == HC_TREENODE) {
            struct TreeNode* n = q.items[i];
            hc_visit(&q, n->left);
            hc_visit(&q, n->right);
        }
#ifdef HC_GRAPH
        else if (kind == HC_NODE) {
            struct Node* n = q.items[i];
            hc_check(n->numNeighbors >= 0 && (!n->numNeighbors || n->neighbors),
                     "invalid neighbors");
            for (int k = 0; k < n->numNeighbors; k++)
                hc_visit(&q, n->neighbors[k]);
        }
#endif
#ifdef HC_NARY
        else if (kind == HC_NODE) {
            struct Node* n = q.items[i];
            hc_check(n->numChildren >= 0 && (!n->numChildren || n->children),
                     "invalid children");
            for (int k = 0; k < n->numChildren; k++)
                hc_visit(&q, n->children[k]);
        }
#endif
#ifdef HC_RANDOM
        else if (kind == HC_NODE) {
            struct Node* n = q.items[i];
            hc_visit(&q, n->next);
            hc_visit(&q, n->random);
        }
#endif
#ifdef HC_NEXT
        else if (kind == HC_NODE) {
            struct Node* n = q.items[i];
            hc_visit(&q, n->left);
            hc_visit(&q, n->right);
            hc_visit(&q, n->next);
        }
#endif
#ifdef HC_QUAD
        else if (kind == HC_NODE) {
            struct Node* n = q.items[i];
            hc_visit(&q, n->topLeft);
            hc_visit(&q, n->topRight);
            hc_visit(&q, n->bottomLeft);
            hc_visit(&q, n->bottomRight);
        }
#endif
#ifdef HC_MULTI
        else if (kind == HC_NODE) {
            struct Node* n = q.items[i];
            hc_visit(&q, n->prev);
            hc_visit(&q, n->next);
            hc_visit(&q, n->child);
        }
#endif
    }
    return q;
}
static void hc_cycle(hc_nodes ns, int pos) {
    if (pos >= 0) {
        hc_check(ns.size && (size_t)pos < ns.size, "invalid cycle index");
        ((struct ListNode*)ns.items[ns.size - 1])->next = ns.items[pos];
    }
}
static hc_value hc_join(hc_value a, hc_value b, int skip, int at) {
    hc_nodes na = hc_collect(a.data, HC_LISTNODE),
             nb = hc_collect(b.data, HC_LISTNODE);
    hc_check(skip >= 0 && at >= 0 && (size_t)skip <= na.size &&
                 (size_t)at <= nb.size,
             "invalid joined list");
    void* tail = (size_t)at < nb.size ? nb.items[at] : NULL;
    if (skip)
        ((struct ListNode*)na.items[skip - 1])->next = tail;
    else
        a.data = tail;
    return a;
}
static void* hc_ref(hc_nodes ns, int val) {
    for (size_t i = 0; i < ns.size; i++) {
        int v = ns.kind == HC_LISTNODE ? ((struct ListNode*)ns.items[i])->val
                                       : ((struct TreeNode*)ns.items[i])->val;
        if (v == val) return ns.items[i];
    }
    hc_fail("referenced node not found");
    return NULL;
}
static yyjson_mut_val* hc_identity(void* ptr, hc_nodes ns, int kind) {
    if (!ptr) return hc_null();
    hc_seen* item;
    HASH_FIND_PTR(ns.seen, &ptr, item);
    hc_check(item != NULL, "returned node is outside the input structure");
    return hc_int(kind == HC_LISTNODE ? (long long)item->index
                                      : ((struct TreeNode*)ptr)->val);
}
static void hc_check_copy(void* ptr, int kind, hc_nodes before) {
    hc_nodes after = hc_collect(ptr, kind);
    for (size_t i = 0; i < after.size; i++) {
        hc_seen* item;
        void* p = after.items[i];
        HASH_FIND_PTR(before.seen, &p, item);
        hc_check(item == NULL, "returned copy shares a node with the input");
    }
}
static void hc_trim(yyjson_mut_val* a) {
    // Mutable arrays are linked lists: removing each tail separately is quadratic.
    size_t i, len, keep = 0;
    yyjson_mut_val* v;
    yyjson_mut_arr_foreach(a, i, len, v) {
        if (!yyjson_mut_is_null(v)) keep = i + 1;
    }
    if (keep < len) yyjson_mut_arr_remove_range(a, keep, len - keep);
}
static hc_value hc_decode(yyjson_val*, int, int);
static yyjson_mut_val* hc_encode(hc_value, int, int);
static struct NestedInteger* hc_nested(yyjson_val* v) {
    struct NestedInteger* n = NestedIntegerInit();
    if (yyjson_is_int(v)) {
        NestedIntegerSetInteger(n, yyjson_get_int(v));
    } else {
        size_t i, max;
        yyjson_val* x;
        yyjson_arr_foreach(v, i, max, x) NestedIntegerAdd(n, hc_nested(x));
    }
    return n;
}
static yyjson_mut_val* hc_nested_out(struct NestedInteger* n, hc_nodes* path) {
    hc_check(n && hc_seen_add(path, n), "NestedInteger is null or cyclic");
    yyjson_mut_val* v;
    if (n->integer)
        v = hc_int(n->value);
    else {
        hc_check(n->size >= 0 && (!n->size || n->list), "invalid nested list");
        v = hc_arr();
        for (int i = 0; i < n->size; i++)
            hc_add(v, hc_nested_out(n->list[i], path));
    }
    hc_seen* item;
    void* p = n;
    HASH_FIND_PTR(path->seen, &p, item);
    HASH_DEL(path->seen, item);
    free(item);
    return v;
}
static void* hc_decode_node(yyjson_val* v, int kind) {
    if (yyjson_is_null(v) || !yyjson_arr_size(v)) return NULL;
    if (kind == HC_LISTNODE) {
        struct ListNode dummy = {0}, *tail = &dummy;
        size_t i, max;
        yyjson_val* x;
        yyjson_arr_foreach(v, i, max, x) {
            tail->next = hc_alloc(1, sizeof(*tail));
            tail = tail->next;
            tail->val = yyjson_get_int(x);
        }
        return dummy.next;
    }
    if (kind == HC_TREENODE) {
        if (yyjson_is_null(yyjson_arr_get(v, 0))) return NULL;
        struct TreeNode* root = hc_alloc(1, sizeof(*root));
        root->val = yyjson_get_int(yyjson_arr_get(v, 0));
        hc_nodes q = {0};
        hc_push(&q, root);
        size_t j = 1;
        for (size_t i = 0; i < q.size && j < yyjson_arr_size(v); i++) {
            struct TreeNode* n = q.items[i];
            for (int k = 0; k < 2 && j < yyjson_arr_size(v); k++, j++) {
                yyjson_val* x = yyjson_arr_get(v, j);
                if (yyjson_is_null(x)) continue;
                struct TreeNode* c = hc_alloc(1, sizeof(*c));
                c->val = yyjson_get_int(x);
                if (k)
                    n->right = c;
                else
                    n->left = c;
                hc_push(&q, c);
            }
        }
        return root;
    }
#ifdef HC_GRAPH
    size_t len = yyjson_arr_size(v);
    struct Node** nodes = hc_alloc(len, sizeof(*nodes));
    for (size_t i = 0; i < len; i++) {
        nodes[i] = hc_alloc(1, sizeof(**nodes));
        nodes[i]->val = (int)i + 1;
    }
    size_t i, count;
    yyjson_val* a;
    yyjson_arr_foreach(v, i, count, a) {
        nodes[i]->numNeighbors = hc_size(yyjson_arr_size(a));
        nodes[i]->neighbors = hc_alloc(yyjson_arr_size(a), sizeof(*nodes));
        for (int j = 0; j < nodes[i]->numNeighbors; j++) {
            int val = yyjson_get_int(yyjson_arr_get(a, j));
            hc_check(val > 0 && (size_t)val <= len, "invalid neighbor");
            nodes[i]->neighbors[j] = nodes[val - 1];
        }
    }
    return nodes[0];
#endif
#ifdef HC_NARY
    struct Node* root = hc_alloc(1, sizeof(*root));
    root->val = yyjson_get_int(yyjson_arr_get(v, 0));
    hc_nodes q = {0};
    hc_push(&q, root);
    size_t j = 2, len = yyjson_arr_size(v);
    for (size_t i = 0; i < q.size && j < len; i++) {
        struct Node* n = q.items[i];
        while (j < len && !yyjson_is_null(yyjson_arr_get(v, j))) {
            struct Node* c = hc_alloc(1, sizeof(*c));
            c->val = yyjson_get_int(yyjson_arr_get(v, j++));
            n->children =
                hc_resize(n->children, (size_t)n->numChildren + 1, sizeof(c));
            n->children[n->numChildren++] = c;
            hc_push(&q, c);
        }
        j++;
    }
    return root;
#endif
#ifdef HC_RANDOM
    size_t len = yyjson_arr_size(v);
    struct Node** nodes = hc_alloc(len, sizeof(*nodes));
    size_t i, count;
    yyjson_val* row;
    yyjson_arr_foreach(v, i, count, row) {
        nodes[i] = hc_alloc(1, sizeof(**nodes));
        nodes[i]->val = yyjson_get_int(yyjson_arr_get(row, 0));
        if (i) nodes[i - 1]->next = nodes[i];
    }
    yyjson_arr_foreach(v, i, count, row) {
        yyjson_val* r = yyjson_arr_get(row, 1);
        if (!yyjson_is_null(r)) {
            int j = yyjson_get_int(r);
            hc_check(j >= 0 && (size_t)j < len, "invalid random pointer");
            nodes[i]->random = nodes[j];
        }
    }
    return nodes[0];
#endif
#if defined(HC_NEXT) || defined(HC_QUAD)
    yyjson_arr_iter iter = yyjson_arr_iter_with(v);
    yyjson_val* first = yyjson_arr_iter_next(&iter);
    if (yyjson_is_null(first)) return NULL;
    hc_nodes q = {0};
    struct Node* root = hc_alloc(1, sizeof(*root));
    hc_push(&q, root);
#ifdef HC_NEXT
    root->val = yyjson_get_int(first);
#else
    root->isLeaf = yyjson_get_int(yyjson_arr_get(first, 0)) != 0;
    root->val = yyjson_get_int(yyjson_arr_get(first, 1)) != 0;
#endif
    for (size_t i = 0; i < q.size && yyjson_arr_iter_has_next(&iter); i++) {
        struct Node* n = q.items[i];
#ifdef HC_NEXT
        struct Node** links[] = {&n->left, &n->right};
#else
        struct Node** links[] = {&n->topLeft, &n->topRight, &n->bottomLeft,
                                 &n->bottomRight};
#endif
        for (size_t k = 0;
             k < sizeof(links) / sizeof(*links) && yyjson_arr_iter_has_next(&iter);
             k++) {
            yyjson_val* x = yyjson_arr_iter_next(&iter);
            if (yyjson_is_null(x)) continue;
            struct Node* c = hc_alloc(1, sizeof(*c));
#ifdef HC_NEXT
            c->val = yyjson_get_int(x);
#else
            c->isLeaf = yyjson_get_int(yyjson_arr_get(x, 0)) != 0;
            c->val = yyjson_get_int(yyjson_arr_get(x, 1)) != 0;
#endif
            *links[k] = c;
            hc_push(&q, c);
        }
    }
    return root;
#endif
#ifdef HC_MULTI
    struct Node* root = NULL;
    hc_nodes above = {0};
    size_t i = 0, len = yyjson_arr_size(v);
    while (i < len) {
        struct Node* parent = NULL;
        if (above.size) {
            i++;
            size_t k = 0;
            while (i < len && yyjson_is_null(yyjson_arr_get(v, i))) {
                i++;
                k++;
            }
            if (i == len) break;
            hc_check(k < above.size, "invalid multilevel list");
            parent = above.items[k];
        }
        hc_nodes level = {0};
        while (i < len && !yyjson_is_null(yyjson_arr_get(v, i))) {
            struct Node* n = hc_alloc(1, sizeof(*n));
            n->val = yyjson_get_int(yyjson_arr_get(v, i++));
            if (level.size) {
                n->prev = level.items[level.size - 1];
                n->prev->next = n;
            }
            hc_push(&level, n);
        }
        hc_check(level.size > 0, "invalid multilevel list");
        if (parent)
            parent->child = level.items[0];
        else
            root = level.items[0];
        above = level;
    }
    return root;
#endif
    return NULL;
}
static yyjson_mut_val* hc_encode_node(void* ptr, int kind) {
    yyjson_mut_val* a = hc_arr();
    if (!ptr) return a;
    hc_nodes seen = {0};
    if (kind == HC_LISTNODE) {
        for (struct ListNode* n = ptr; n; n = n->next) {
            hc_check(hc_visit(&seen, n), "list has a cycle");
            hc_add(a, hc_int(n->val));
        }
        return a;
    }
    if (kind == HC_TREENODE) {
        hc_nodes q = {0};
        hc_push(&q, ptr);
        for (size_t i = 0; i < q.size; i++) {
            struct TreeNode* n = q.items[i];
            if (!n) {
                hc_add(a, hc_null());
                continue;
            }
            hc_check(hc_visit(&seen, n), "tree has a cycle or shared node");
            hc_add(a, hc_int(n->val));
            hc_push(&q, n->left);
            hc_push(&q, n->right);
        }
        hc_trim(a);
        return a;
    }
#ifdef HC_GRAPH
    hc_nodes q = hc_collect(ptr, HC_NODE);
    hc_check(((struct Node*)ptr)->val == 1, "graph must start at node 1");
    struct Node** byval = hc_alloc(q.size, sizeof(*byval));
    for (size_t i = 0; i < q.size; i++) {
        struct Node* n = q.items[i];
        hc_check(n->val > 0 && (size_t)n->val <= q.size && !byval[n->val - 1],
                 "invalid graph node values");
        byval[n->val - 1] = n;
    }
    for (size_t i = 0; i < q.size; i++) {
        struct Node* n = byval[i];
        yyjson_mut_val* row = hc_arr();
        for (int j = 0; j < n->numNeighbors; j++) {
            hc_check(n->neighbors[j] != NULL, "null graph neighbor");
            hc_add(row, hc_int(n->neighbors[j]->val));
        }
        hc_add(a, row);
    }
    return a;
#endif
#ifdef HC_NARY
    struct Node* root = ptr;
    hc_nodes q = {0};
    hc_visit(&q, root);
    hc_add(a, hc_int(root->val));
    hc_add(a, hc_null());
    for (size_t i = 0; i < q.size; i++) {
        struct Node* n = q.items[i];
        hc_check(n->numChildren >= 0 && (!n->numChildren || n->children),
                 "invalid children");
        for (int j = 0; j < n->numChildren; j++) {
            struct Node* c = n->children[j];
            hc_check(c && hc_visit(&q, c),
                     "n-ary tree has null, cyclic or shared child");
            hc_add(a, hc_int(c->val));
        }
        hc_add(a, hc_null());
    }
    hc_trim(a);
    return a;
#endif
#ifdef HC_RANDOM
    hc_nodes q = {0};
    for (struct Node* n = ptr; n; n = n->next)
        hc_check(hc_visit(&q, n), "random list has a cycle");
    for (size_t i = 0; i < q.size; i++) {
        struct Node* n = q.items[i];
        yyjson_mut_val* row = hc_arr();
        hc_add(row, hc_int(n->val));
        if (!n->random)
            hc_add(row, hc_null());
        else {
            hc_seen* item;
            void* p = n->random;
            HASH_FIND_PTR(q.seen, &p, item);
            hc_check(item != NULL, "random pointer is outside returned list");
            hc_add(row, hc_int((long long)item->index));
        }
        hc_add(a, row);
    }
    return a;
#endif
#ifdef HC_NEXT
    struct Node* head = ptr;
    while (head) {
        yyjson_mut_val* row = hc_arr();
        struct Node* next = NULL;
        for (struct Node* n = head; n; n = n->next) {
            hc_check(hc_visit(&seen, n),
                     "next pointers contain a cycle or shared node");
            hc_add(row, hc_int(n->val));
            if (!next) next = n->left ? n->left : n->right;
        }
        hc_add(a, row);
        head = next;
    }
    return a;
#endif
#ifdef HC_QUAD
    hc_nodes q = {0};
    hc_push(&q, ptr);
    for (size_t i = 0; i < q.size; i++) {
        struct Node* n = q.items[i];
        if (!n) {
            hc_add(a, hc_null());
            continue;
        }
        hc_check(hc_visit(&seen, n), "quad tree has a cycle or shared node");
        yyjson_mut_val* row = hc_arr();
        hc_add(row, hc_int(n->isLeaf ? 1 : 0));
        hc_add(row, hc_int(n->isLeaf ? (n->val ? 1 : 0) : 1));
        hc_add(a, row);
        hc_push(&q, n->topLeft);
        hc_push(&q, n->topRight);
        hc_push(&q, n->bottomLeft);
        hc_push(&q, n->bottomRight);
    }
    hc_trim(a);
    return a;
#endif
#ifdef HC_MULTI
    struct Node* head = ptr;
    while (head) {
        struct Node *child = NULL, *prev = NULL;
        size_t at = 0, i = 0;
        for (struct Node* n = head; n; prev = n, n = n->next, i++) {
            hc_check(hc_visit(&seen, n), "multilevel list has a cycle");
            hc_check(n->prev == prev, "incorrect prev pointer");
            hc_add(a, hc_int(n->val));
            if (n->child) {
                hc_check(!child, "multiple child levels cannot be encoded");
                child = n->child;
                at = i;
            }
        }
        if (child) {
            hc_add(a, hc_null());
            while (at--) hc_add(a, hc_null());
        }
        head = child;
    }
    return a;
#endif
    hc_fail("unknown node type");
    return NULL;
}
static size_t hc_width(int kind) {
    switch (kind) {
        case HC_INTEGER:
            return sizeof(int);
        case HC_LONG:
            return sizeof(long long);
        case HC_DOUBLE:
            return sizeof(double);
        case HC_BOOLEAN:
            return sizeof(bool);
        case HC_CHARACTER:
            return sizeof(char);
        default:
            return sizeof(void*);
    }
}
static hc_value hc_decode(yyjson_val* v, int kind, int depth) {
    hc_value out = {0};
    if (depth) {
        out.size = hc_size(yyjson_arr_size(v));
        out.data =
            hc_alloc((size_t)out.size + (depth == 1 && kind == HC_CHARACTER),
                     depth > 1 ? sizeof(void*) : hc_width(kind));
        if (depth > 1) out.rows = hc_alloc(out.size, sizeof(int));
        if (depth > 2) out.cols = hc_alloc(out.size, sizeof(int*));
        // Nested JSON elements have variable width; indexed lookup scans them.
        size_t i, count;
        yyjson_val* element;
        yyjson_arr_foreach(v, i, count, element) {
            hc_value child = hc_decode(element, kind, depth - 1);
            if (depth > 1) {
                ((void**)out.data)[i] = child.data;
                out.rows[i] = child.size;
                if (depth > 2) out.cols[i] = child.rows;
            } else if (kind >= HC_STRING)
                ((void**)out.data)[i] = child.data;
            else
                memcpy((char*)out.data + (size_t)i * hc_width(kind), child.data,
                       hc_width(kind));
        }
        return out;
    }
    if (kind == HC_STRING) {
        out.data = hc_string(v);
        return out;
    }
    if (kind == HC_LISTNODE || kind == HC_TREENODE || kind == HC_NODE) {
        out.data = hc_decode_node(v, kind);
        return out;
    }
    if (kind == HC_NESTEDINTEGER) {
        out.data = hc_nested(v);
        return out;
    }
    if (kind == HC_EMPLOYEE) {
        struct Employee* n = hc_alloc(1, sizeof(*n));
        n->id = yyjson_get_int(yyjson_arr_get(v, 0));
        n->importance = yyjson_get_int(yyjson_arr_get(v, 1));
        hc_value subs = hc_decode(yyjson_arr_get(v, 2), HC_INTEGER, 1);
        n->subordinates = subs.data;
        n->subordinatesSize = subs.size;
        out.data = n;
        return out;
    }
    out.data = hc_alloc(1, hc_width(kind));
    switch (kind) {
        case HC_INTEGER:
            *(int*)out.data = yyjson_get_int(v);
            break;
        case HC_LONG:
            hc_check(yyjson_is_int(v) && (!yyjson_is_uint(v) ||
                                          yyjson_get_uint(v) <= LLONG_MAX),
                     "long outside signed 64-bit range");
            *(long long*)out.data = yyjson_get_sint(v);
            break;
        case HC_DOUBLE:
            *(double*)out.data = yyjson_get_num(v);
            break;
        case HC_BOOLEAN:
            *(bool*)out.data = yyjson_get_bool(v);
            break;
        case HC_CHARACTER:
            hc_check(yyjson_is_str(v) && yyjson_get_len(v) == 1,
                     "invalid character");
            *(char*)out.data = *yyjson_get_str(v);
            break;
        default:
            break;
    }
    return out;
}
static yyjson_mut_val* hc_encode(hc_value v, int kind, int depth) {
    if (depth) {
        hc_check(v.size >= 0 && (!v.size || v.data),
                 "invalid returned array length or pointer");
        hc_check(depth < 2 || !v.size || v.rows, "missing returned row sizes");
        hc_check(depth < 3 || !v.size || v.cols,
                 "missing returned depth sizes");
        yyjson_mut_val* a = hc_arr();
        for (int i = 0; i < v.size; i++) {
            hc_value child = {0};
            if (depth > 1) {
                child.data = ((void**)v.data)[i];
                child.size = v.rows[i];
                if (depth > 2) child.rows = v.cols[i];
            } else
                child.data = kind >= HC_STRING
                                 ? ((void**)v.data)[i]
                                 : (char*)v.data + (size_t)i * hc_width(kind);
            hc_add(a, hc_encode(child, kind, depth - 1));
        }
        return a;
    }
    switch (kind) {
        case HC_VOID:
            return hc_null();
        case HC_INTEGER:
            return hc_int(*(int*)v.data);
        case HC_LONG:
            return hc_int(*(long long*)v.data);
        case HC_DOUBLE:
            hc_check(isfinite(*(double*)v.data), "non-finite result");
            return yyjson_mut_real(hc_doc, *(double*)v.data);
        case HC_BOOLEAN:
            return yyjson_mut_bool(hc_doc, *(bool*)v.data);
        case HC_CHARACTER:
            hc_check(*(unsigned char*)v.data < 128, "character must be ASCII");
            return yyjson_mut_strncpy(hc_doc, v.data, 1);
        case HC_STRING:
            hc_check(v.data != NULL, "null string result");
            return yyjson_mut_strcpy(hc_doc, v.data);
        case HC_LISTNODE:
        case HC_TREENODE:
        case HC_NODE:
            return hc_encode_node(v.data, kind);
        case HC_NESTEDINTEGER: {
            hc_nodes path = {0};
            return hc_nested_out(v.data, &path);
        }
        case HC_EMPLOYEE: {
            struct Employee* n = v.data;
            hc_check(n != NULL, "null Employee");
            yyjson_mut_val* a = hc_arr();
            hc_add(a, hc_int(n->id));
            hc_add(a, hc_int(n->importance));
            hc_value subs = {.data = n->subordinates,
                             .size = n->subordinatesSize};
            hc_add(a, hc_encode(subs, HC_INTEGER, 1));
            return a;
        }
    }
    hc_fail("unknown type");
    return NULL;
}
static hc_value hc_clone(hc_value v, int kind) {
    yyjson_mut_val* a = hc_encode(v, kind, 0);
    size_t len;
    char* json = yyjson_mut_val_write(a, 0, &len);
    hc_check(json != NULL, "cannot copy structure");
    yyjson_doc* d = yyjson_read(json, len, 0);
    hc_check(d != NULL, "cannot copy structure");
    hc_value out = hc_decode(yyjson_doc_get_root(d), kind, 0);
    yyjson_doc_free(d);
    free(json);
    return out;
}
static _Atomic unsigned long long hc_sequence;
static _Atomic long long hc_started, hc_elapsed;
static pthread_mutex_t hc_result_lock = PTHREAD_MUTEX_INITIALIZER;
static _Atomic bool hc_done;
static long long hc_now(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (long long)t.tv_sec * 1000000000LL + t.tv_nsec;
}
static void hc_start(void) {
    atomic_fetch_add(&hc_sequence, 1);
    atomic_store(&hc_started, hc_now());
    atomic_fetch_add(&hc_sequence, 1);
}
static void hc_stop(void) {
    long long end = hc_now();
    atomic_fetch_add(&hc_sequence, 1);
    long long start = atomic_exchange(&hc_started, 0);
    if (start) atomic_fetch_add(&hc_elapsed, end - start);
    atomic_fetch_add(&hc_sequence, 1);
}
static long long hc_spent(void) {
    for (;;) {
        unsigned long long seq = atomic_load(&hc_sequence);
        if (seq & 1) continue;
        long long elapsed = atomic_load(&hc_elapsed),
                  start = atomic_load(&hc_started);
        if (seq == atomic_load(&hc_sequence))
            return elapsed + (start ? hc_now() - start : 0);
    }
}
static FILE* hc_output;
static int hc_case_id;
static long long hc_limit;
static void* hc_watch(void* unused) {
    (void)unused;
    struct timespec pause = {0, 1000000};
    while (!atomic_load(&hc_done)) {
        long long elapsed = hc_spent();
        if (elapsed > hc_limit) {
            pthread_mutex_lock(&hc_result_lock);
            if (!atomic_load(&hc_done)) {
                fprintf(hc_output,
                        "{\"id\":%d,\"time_limit\":true,\"elapsed_ns\":%lld}\n",
                        hc_case_id, elapsed);
                fflush(hc_output);
                _Exit(0);
            }
            pthread_mutex_unlock(&hc_result_lock);
            return NULL;
        }
        nanosleep(&pause, NULL);
    }
    return NULL;
}
static yyjson_mut_val* hc_call(yyjson_val*);
static int hc_run(void) {
    struct rlimit stack;
    if (!getrlimit(RLIMIT_STACK, &stack)) {
        rlim_t wanted = 512ULL * 1024 * 1024;
        stack.rlim_cur = stack.rlim_max < wanted ? stack.rlim_max : wanted;
        setrlimit(RLIMIT_STACK, &stack);
    }
    const char *cases = getenv("HC_CASES"), *output = getenv("HC_OUT");
    if (!cases || !output) return 1;
    yyjson_doc* input = yyjson_read_file(cases, 0, NULL, NULL);
    if (!input) return 1;
    yyjson_val* root = yyjson_doc_get_root(input);
    if (yyjson_arr_size(root) != 1) return 1;
    yyjson_val* c = yyjson_arr_get(root, 0);
    hc_case_id = yyjson_get_int(yyjson_obj_get(c, "id"));
    hc_output = fopen(output, "w");
    if (!hc_output) return 1;
    hc_doc = yyjson_mut_doc_new(NULL);
    if (!hc_doc) return 1;
    const char* limit = getenv("HC_TIME_LIMIT_NS");
    pthread_t watcher;
    bool watching = false;
    if (limit) {
        hc_limit = strtoll(limit, NULL, 10);
        if (pthread_create(&watcher, NULL, hc_watch, NULL)) return 1;
        watching = true;
    }
    yyjson_mut_val* res = yyjson_mut_obj(hc_doc);
    yyjson_mut_obj_add_int(hc_doc, res, "id", hc_case_id);
    if (!setjmp(hc_jump)) {
        yyjson_mut_val* val = hc_call(yyjson_obj_get(c, "input"));
        hc_check(yyjson_mut_obj_add_val(hc_doc, res, "output", val),
                 "cannot encode result");
    } else {
        hc_stop();
        yyjson_mut_obj_add_strcpy(hc_doc, res, "error", hc_error);
    }
    yyjson_mut_obj_add_sint(hc_doc, res, "elapsed_ns",
                            atomic_load(&hc_elapsed));
    size_t len;
    char* text = yyjson_mut_val_write(res, 0, &len);
    pthread_mutex_lock(&hc_result_lock);
    atomic_store(&hc_done, true);
    if (text) {
        fwrite(text, 1, len, hc_output);
        fputc('\n', hc_output);
    } else {
        fprintf(hc_output,
                "{\"id\":%d,\"error\":\"invalid UTF-8 or JSON result\"}\n",
                hc_case_id);
    }
    fflush(hc_output);
    pthread_mutex_unlock(&hc_result_lock);
    if (watching) pthread_join(watcher, NULL);
    fclose(hc_output);
    return 0;
}
#endif
