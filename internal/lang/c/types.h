#ifndef HC_C_TYPES_H
#define HC_C_TYPES_H
#include <assert.h>
#include <ctype.h>
#include <errno.h>
#include <float.h>
#include <inttypes.h>
#include <limits.h>
#include <math.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <uthash.h>
#ifdef HC_GLIB
#include <glib.h>
#endif
_Static_assert(CHAR_BIT == 8 && INT_MAX == 2147483647 &&
                   LLONG_MAX == 9223372036854775807LL,
               "hc requires 32-bit int and 64-bit long long");
struct ListNode {
    int val;
    struct ListNode* next;
};
struct TreeNode {
    int val;
    struct TreeNode *left, *right;
};
struct Employee {
    int id, importance;
    int* subordinates;
    int subordinatesSize;
};
struct NestedInteger;
struct NestedInteger* NestedIntegerInit(void);
bool NestedIntegerIsInteger(struct NestedInteger*);
int NestedIntegerGetInteger(struct NestedInteger*);
void NestedIntegerSetInteger(struct NestedInteger*, int);
void NestedIntegerAdd(struct NestedInteger*, struct NestedInteger*);
struct NestedInteger** NestedIntegerGetList(struct NestedInteger*);
int NestedIntegerGetListSize(struct NestedInteger*);
#endif
