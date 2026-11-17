class Solution {
   public:
    Node* connect(Node* root) {
        vector<Node*> level;
        if (root) level.push_back(root);
        while (!level.empty()) {
            vector<Node*> next;
            for (size_t i = 0; i < level.size(); ++i) {
                auto n = level[i];
                n->next = i + 1 < level.size() ? level[i + 1] : nullptr;
                if (n->left) next.push_back(n->left);
                if (n->right) next.push_back(n->right);
            }
            level = move(next);
        }
        return root;
    }
};
