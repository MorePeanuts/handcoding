class Solution {
   public:
    vector<int> preorder(Node* root) {
        vector<int> out;
        vector<Node*> todo;
        if (root) todo.push_back(root);
        while (!todo.empty()) {
            auto n = todo.back();
            todo.pop_back();
            out.push_back(n->val);
            for (auto it = n->children.rbegin(); it != n->children.rend(); ++it)
                todo.push_back(*it);
        }
        return out;
    }
};
