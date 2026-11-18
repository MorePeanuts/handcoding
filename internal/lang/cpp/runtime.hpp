namespace hc {
constexpr size_t max_nodes = 1000000;
inline void trim(J& a) {
    while (!a.empty() && a.back().is_null()) a.erase(a.end() - 1);
}
template <class T>
struct Codec {
    static T read(const J& j) { return j.get<T>(); }
    static J write(const T& v) { return J(v); }
};
template <>
struct Codec<char> {
    static char read(const J& j) {
        auto s = j.get<string>();
        check(s.size() == 1 && static_cast<unsigned char>(s[0]) < 128,
              "character must be ASCII");
        return s[0];
    }
    static J write(char c) {
        check(static_cast<unsigned char>(c) < 128, "character must be ASCII");
        return string(1, c);
    }
};
template <class T>
struct Codec<vector<T>> {
    static vector<T> read(const J& j) {
        vector<T> v;
        v.reserve(j.size());
        for (auto& x : j) v.push_back(Codec<T>::read(x));
        return v;
    }
    static J write(const vector<T>& v) {
        J a = J::array();
        for (const auto& x : v) a.push_back(Codec<T>::write(x));
        return a;
    }
};
template <class T>
J encode(const T& v) {
    return Codec<T>::write(v);
}
template <class T>
T decode(const J& j) {
    return Codec<T>::read(j);
}
inline vector<ListNode*> nodes(ListNode* n) {
    vector<ListNode*> a;
    unordered_set<ListNode*> seen;
    while (n && seen.insert(n).second) {
        check(a.size() < max_nodes, "list too large");
        a.push_back(n);
        n = n->next;
    }
    return a;
}
template <class T>
vector<T*> tree_nodes(T* n) {
    vector<T*> a;
    if (n) a.push_back(n);
    unordered_set<T*> seen;
    for (size_t i = 0; i < a.size(); ++i) {
        check(a.size() <= max_nodes && seen.insert(a[i]).second,
              "tree has a cycle or shared node");
        if (a[i]->left) a.push_back(a[i]->left);
        if (a[i]->right) a.push_back(a[i]->right);
    }
    return a;
}
inline vector<TreeNode*> nodes(TreeNode* n) { return tree_nodes(n); }
template <>
struct Codec<ListNode*> {
    static ListNode* read(const J& j) {
        ListNode dummy;
        auto* tail = &dummy;
        for (auto& x : j) {
            tail->next = new ListNode(x.get<int>());
            tail = tail->next;
        }
        return dummy.next;
    }
    static J write(ListNode* n) {
        J a = J::array();
        unordered_set<ListNode*> seen;
        while (n) {
            check(a.size() < max_nodes && seen.insert(n).second,
                  "list has a cycle");
            a.push_back(n->val);
            n = n->next;
        }
        return a;
    }
};
template <class T>
T* read_tree(const J& j) {
    if (j.empty() || j[0].is_null()) return nullptr;
    T* root = new T(j[0].get<int>());
    vector<T*> q{root};
    size_t k = 1;
    for (size_t i = 0; k < j.size(); ++i) {
        check(i < q.size(), "invalid tree");
        for (T** p : {&q[i]->left, &q[i]->right}) {
            if (k < j.size() && !j[k].is_null()) {
                *p = new T(j[k].get<int>());
                q.push_back(*p);
            }
            ++k;
        }
    }
    return root;
}
template <class T>
J write_tree(T* root) {
    J a = J::array();
    vector<T*> q{root};
    unordered_set<T*> seen;
    for (size_t i = 0; i < q.size(); ++i) {
        T* n = q[i];
        check(q.size() <= max_nodes * 2 + 1, "tree too large");
        if (!n) {
            a.push_back(nullptr);
            continue;
        }
        check(seen.insert(n).second, "tree has a cycle or shared node");
        a.push_back(n->val);
        q.push_back(n->left);
        q.push_back(n->right);
    }
    trim(a);
    return a;
}
template <>
struct Codec<TreeNode*> {
    static TreeNode* read(const J& j) { return read_tree<TreeNode>(j); }
    static J write(TreeNode* n) { return write_tree(n); }
};
template <>
struct Codec<NestedInteger> {
    static NestedInteger read(const J& j) {
        if (j.is_number_integer()) return NestedInteger(j.get<int>());
        NestedInteger n;
        for (auto& x : j) n.add(read(x));
        return n;
    }
    static J write(const NestedInteger& n) {
        if (n.isInteger()) return n.getInteger();
        J a = J::array();
        for (auto& x : n.getList()) a.push_back(write(x));
        return a;
    }
};
template <>
struct Codec<Employee*> {
    static Employee* read(const J& j) {
        auto* e = new Employee;
        e->id = j[0].get<int>();
        e->importance = j[1].get<int>();
        e->subordinates = j[2].get<vector<int>>();
        return e;
    }
    static J write(Employee* e) {
        check(e != nullptr, "null employee");
        return J::array({e->id, e->importance, e->subordinates});
    }
};
#ifdef HC_GRAPH
inline vector<Node*> links(Node* n) { return n->neighbors; }
template <>
struct Codec<Node*> {
    static Node* read(const J& j) {
        vector<Node*> ns;
        for (size_t i = 0; i < j.size(); ++i)
            ns.push_back(new Node(static_cast<int>(i + 1)));
        for (size_t i = 0; i < j.size(); ++i)
            for (auto& x : j[i])
                ns[i]->neighbors.push_back(ns.at(x.get<size_t>() - 1));
        return ns.empty() ? nullptr : ns[0];
    }
    static J write(Node* root) {
        J a = J::array();
        if (!root) return a;
        vector<Node*> q{root};
        unordered_set<Node*> seen{root};
        map<int, Node*> byval;
        for (size_t i = 0; i < q.size(); ++i) {
            Node* n = q[i];
            check(q.size() <= max_nodes, "graph too large");
            check(n->val > 0 && byval.emplace(n->val, n).second,
                  "duplicate or invalid graph node value");
            for (Node* c : n->neighbors) {
                check(c != nullptr, "null neighbor");
                if (seen.insert(c).second) q.push_back(c);
            }
        }
        check(root->val == 1, "graph root must be node 1");
        for (size_t i = 1; i <= q.size(); ++i) {
            check(byval.count(static_cast<int>(i)),
                  "graph node values must be 1 to n");
            J row = J::array();
            for (Node* n : byval[static_cast<int>(i)]->neighbors)
                row.push_back(n->val);
            a.push_back(row);
        }
        return a;
    }
};
#endif
#ifdef HC_NARY
inline vector<Node*> links(Node* n) { return n->children; }
template <>
struct Codec<Node*> {
    static Node* read(const J& j) {
        if (j.empty()) return nullptr;
        Node* root = new Node(j[0].get<int>());
        vector<Node*> q{root};
        size_t k = 2;
        for (size_t i = 0; k < j.size(); ++i) {
            check(i < q.size(), "invalid N-ary tree");
            while (k < j.size() && !j[k].is_null()) {
                Node* c = new Node(j[k++].get<int>());
                q[i]->children.push_back(c);
                q.push_back(c);
            }
            ++k;
        }
        return root;
    }
    static J write(Node* root) {
        J a = J::array();
        if (!root) return a;
        a.push_back(root->val);
        a.push_back(nullptr);
        vector<Node*> q{root};
        unordered_set<Node*> seen{root};
        for (size_t i = 0; i < q.size(); ++i) {
            for (Node* c : q[i]->children) {
                check(c && q.size() < max_nodes && seen.insert(c).second,
                      "invalid N-ary child or cycle");
                a.push_back(c->val);
                q.push_back(c);
            }
            a.push_back(nullptr);
        }
        trim(a);
        return a;
    }
};
#endif
#ifdef HC_RANDOM
inline vector<Node*> links(Node* n) { return {n->next, n->random}; }
template <>
struct Codec<Node*> {
    static Node* read(const J& j) {
        vector<Node*> ns;
        for (auto& x : j) ns.push_back(new Node(x[0].get<int>()));
        for (size_t i = 0; i < ns.size(); ++i) {
            if (i + 1 < ns.size()) ns[i]->next = ns[i + 1];
            if (!j[i][1].is_null())
                ns[i]->random = ns.at(j[i][1].get<size_t>());
        }
        return ns.empty() ? nullptr : ns[0];
    }
    static J write(Node* root) {
        vector<Node*> ns;
        unordered_map<Node*, size_t> index;
        for (Node* n = root; n; n = n->next) {
            check(ns.size() < max_nodes && index.emplace(n, ns.size()).second,
                  "random list has a cycle");
            ns.push_back(n);
        }
        J a = J::array();
        for (Node* n : ns) {
            J r = nullptr;
            if (n->random) {
                check(index.count(n->random),
                      "random pointer is outside the list");
                r = index[n->random];
            }
            a.push_back(J::array({n->val, r}));
        }
        return a;
    }
};
#endif
#ifdef HC_NEXT
inline vector<Node*> links(Node* n) { return {n->left, n->right, n->next}; }
template <>
struct Codec<Node*> {
    static Node* read(const J& j) { return read_tree<Node>(j); }
    static J write(Node* root) {
        J a = J::array();
        vector<Node*> level;
        if (root) level.push_back(root);
        unordered_set<Node*> seen;
        while (!level.empty()) {
            J row = J::array();
            vector<Node*> next;
            for (size_t i = 0; i < level.size(); ++i) {
                Node* n = level[i];
                check(seen.size() < max_nodes && seen.insert(n).second,
                      "next tree has a cycle");
                check(
                    n->next == (i + 1 < level.size() ? level[i + 1] : nullptr),
                    "incorrect next pointer");
                row.push_back(n->val);
                if (n->left) next.push_back(n->left);
                if (n->right) next.push_back(n->right);
            }
            a.push_back(row);
            level = move(next);
        }
        return a;
    }
};
#endif
#ifdef HC_QUAD
inline vector<Node*> links(Node* n) {
    return {n->topLeft, n->topRight, n->bottomLeft, n->bottomRight};
}
template <>
struct Codec<Node*> {
    static Node* read(const J& j) {
        if (j.empty() || j[0].is_null()) return nullptr;
        auto make = [](const J& x) {
            return new Node(x[1].get<int>() != 0, x[0].get<int>() != 0);
        };
        Node* root = make(j[0]);
        vector<Node*> q{root};
        size_t k = 1;
        for (size_t i = 0; k < j.size(); ++i) {
            check(i < q.size(), "invalid quad tree");
            Node* n = q[i];
            for (Node** c :
                 {&n->topLeft, &n->topRight, &n->bottomLeft, &n->bottomRight}) {
                if (k < j.size() && !j[k].is_null()) {
                    *c = make(j[k]);
                    q.push_back(*c);
                }
                ++k;
            }
        }
        return root;
    }
    static J write(Node* root) {
        J a = J::array();
        vector<Node*> q{root};
        unordered_set<Node*> seen;
        for (size_t i = 0; i < q.size(); ++i) {
            Node* n = q[i];
            if (!n) {
                a.push_back(nullptr);
                continue;
            }
            check(seen.size() < max_nodes && seen.insert(n).second,
                  "quad tree has a cycle or shared node");
            a.push_back(J::array(
                {n->isLeaf ? 1 : 0, n->isLeaf ? (n->val ? 1 : 0) : 1}));
            for (Node* c : links(n)) q.push_back(c);
        }
        trim(a);
        return a;
    }
};
#endif
#ifdef HC_MULTI
inline vector<Node*> links(Node* n) { return {n->prev, n->next, n->child}; }
template <>
struct Codec<Node*> {
    static Node* read(const J& j) {
        Node* root = nullptr;
        vector<Node*> above;
        size_t i = 0;
        while (i < j.size()) {
            Node* parent = nullptr;
            if (!above.empty()) {
                ++i;
                size_t k = 0;
                while (i < j.size() && j[i].is_null()) {
                    ++i;
                    ++k;
                }
                if (i == j.size()) break;
                parent = above.at(k);
            }
            vector<Node*> level;
            while (i < j.size() && !j[i].is_null()) {
                Node* n = new Node(j[i++].get<int>());
                if (!level.empty()) {
                    n->prev = level.back();
                    level.back()->next = n;
                }
                level.push_back(n);
            }
            check(!level.empty(), "invalid multilevel list");
            if (parent)
                parent->child = level[0];
            else
                root = level[0];
            above = move(level);
        }
        return root;
    }
    static J write(Node* head) {
        J a = J::array();
        unordered_set<Node*> seen;
        while (head) {
            Node* child = nullptr;
            size_t at = 0, i = 0;
            Node* prev = nullptr;
            for (Node* n = head; n; prev = n, n = n->next, ++i) {
                check(seen.size() < max_nodes && seen.insert(n).second,
                      "multilevel list has a cycle");
                check(n->prev == prev, "incorrect prev pointer");
                a.push_back(n->val);
                if (n->child) {
                    check(!child, "multiple child levels cannot be encoded");
                    child = n->child;
                    at = i;
                }
            }
            if (child) {
                a.push_back(nullptr);
                while (at--) a.push_back(nullptr);
            }
            head = child;
        }
        return a;
    }
};
#endif
#if defined(HC_GRAPH) || defined(HC_NARY) || defined(HC_RANDOM) || \
    defined(HC_NEXT) || defined(HC_QUAD) || defined(HC_MULTI)
inline unordered_set<Node*> node_set(Node* root) {
    unordered_set<Node*> seen;
    vector<Node*> q;
    if (root) {
        q.push_back(root);
        seen.insert(root);
    }
    for (size_t i = 0; i < q.size(); ++i) {
        check(q.size() <= max_nodes, "structure too large");
        for (Node* n : links(q[i]))
            if (n && seen.insert(n).second) q.push_back(n);
    }
    return seen;
}
inline void check_copy(Node* out, const unordered_set<Node*>& before) {
    for (Node* n : node_set(out))
        check(!before.count(n), "returned copy shares a node with the input");
}
#endif
inline vector<ListNode*> links(ListNode* n) { return {n->next}; }
inline vector<TreeNode*> links(TreeNode* n) { return {n->left, n->right}; }
template <class T>
unordered_set<T*> node_set(T* root) {
    unordered_set<T*> seen;
    vector<T*> q;
    if (root) {
        q.push_back(root);
        seen.insert(root);
    }
    for (size_t i = 0; i < q.size(); ++i) {
        check(q.size() <= max_nodes, "structure too large");
        for (T* n : links(q[i]))
            if (n && seen.insert(n).second) q.push_back(n);
    }
    return seen;
}
template <class T>
void check_copy(T* out, const unordered_set<T*>& before) {
    for (T* n : node_set(out))
        check(!before.count(n), "returned copy shares a node with the input");
}
template <class T>
T* ref(const vector<T*>& ns, int value) {
    T* out = nullptr;
    for (T* n : ns)
        if (n->val == value) {
            check(!out, "ambiguous node value");
            out = n;
        }
    check(out != nullptr, "node value not found");
    return out;
}
inline J identity(ListNode* n, const vector<ListNode*>& ns) {
    if (!n) return nullptr;
    for (size_t i = 0; i < ns.size(); ++i)
        if (n == ns[i]) return i;
    throw runtime_error("returned node is outside the input list");
}
inline J identity(TreeNode* n, const vector<TreeNode*>& ns) {
    if (!n) return nullptr;
    for (TreeNode* x : ns)
        if (n == x) return x->val;
    throw runtime_error("returned node is outside the input tree");
}
}  // namespace hc
