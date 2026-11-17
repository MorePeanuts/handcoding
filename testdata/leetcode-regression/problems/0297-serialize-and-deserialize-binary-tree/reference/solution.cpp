class Codec {
   public:
    string serialize(TreeNode* root) {
        ostringstream out;
        queue<TreeNode*> q;
        q.push(root);
        while (!q.empty()) {
            auto n = q.front();
            q.pop();
            if (!n) {
                out << "# ";
                continue;
            }
            out << n->val << ' ';
            q.push(n->left);
            q.push(n->right);
        }
        return out.str();
    }
    TreeNode* deserialize(string data) {
        istringstream in(data);
        string s;
        in >> s;
        if (s == "#") return nullptr;
        auto root = new TreeNode(stoi(s));
        queue<TreeNode*> q;
        q.push(root);
        while (!q.empty()) {
            auto n = q.front();
            q.pop();
            for (auto slot : {&n->left, &n->right}) {
                in >> s;
                if (s != "#") {
                    *slot = new TreeNode(stoi(s));
                    q.push(*slot);
                }
            }
        }
        return root;
    }
};
