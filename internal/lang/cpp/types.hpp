#pragma once
#include <algorithm>
#include <array>
#include <bit>
#include <bitset>
#include <cassert>
#include <cctype>
#include <charconv>
#include <climits>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <deque>
#include <functional>
#include <iomanip>
#include <iostream>
#include <iterator>
#include <limits>
#include <list>
#include <map>
#include <memory>
#include <numeric>
#include <optional>
#include <queue>
#include <random>
#include <ranges>
#include <set>
#include <span>
#include <sstream>
#include <stack>
#include <stdexcept>
#include <string>
#include <tuple>
#include <unordered_map>
#include <unordered_set>
#include <utility>
#include <variant>
#include <vector>
using namespace std;
static_assert(sizeof(int) == 4 && sizeof(long long) == 8,
              "hc requires 32-bit int and 64-bit long long");
struct ListNode {
    int val;
    ListNode* next;
    ListNode() : val(0), next(nullptr) {}
    ListNode(int x) : val(x), next(nullptr) {}
    ListNode(int x, ListNode* n) : val(x), next(n) {}
};
struct TreeNode {
    int val;
    TreeNode *left, *right;
    TreeNode() : val(0), left(nullptr), right(nullptr) {}
    TreeNode(int x) : val(x), left(nullptr), right(nullptr) {}
    TreeNode(int x, TreeNode* l, TreeNode* r) : val(x), left(l), right(r) {}
};
class NestedInteger {
    bool scalar = false;
    int value = 0;
    vector<NestedInteger> items;

   public:
    NestedInteger() = default;
    NestedInteger(int v) : scalar(true), value(v) {}
    bool isInteger() const { return scalar; }
    int getInteger() const { return value; }
    void setInteger(int v) {
        scalar = true;
        value = v;
        items.clear();
    }
    void add(const NestedInteger& n) {
        if (scalar) {
            scalar = false;
            items.clear();
        }
        items.push_back(n);
    }
    const vector<NestedInteger>& getList() const { return items; }
    vector<NestedInteger>& getList() { return items; }
};
class Employee {
   public:
    int id = 0, importance = 0;
    vector<int> subordinates;
};
