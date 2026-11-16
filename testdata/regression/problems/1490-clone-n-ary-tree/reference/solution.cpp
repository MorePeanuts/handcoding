class Solution {
   public:
    Node* cloneTree(Node* root) {
        if (!root) return nullptr;
        auto copy = new Node(root->val);
        vector<pair<Node*, Node*>> todo{{root, copy}};
        while (!todo.empty()) {
            auto [a, b] = todo.back();
            todo.pop_back();
            for (auto c : a->children) {
                auto d = new Node(c->val);
                b->children.push_back(d);
                todo.push_back({c, d});
            }
        }
        return copy;
    }
};
