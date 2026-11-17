class Solution {
   public:
    Node* cloneGraph(Node* node) {
        if (!node) return nullptr;
        unordered_map<Node*, Node*> copy;
        copy[node] = new Node(node->val);
        queue<Node*> q;
        q.push(node);
        while (!q.empty()) {
            auto n = q.front();
            q.pop();
            for (auto c : n->neighbors) {
                if (!copy.count(c)) {
                    copy[c] = new Node(c->val);
                    q.push(c);
                }
                copy[n]->neighbors.push_back(copy[c]);
            }
        }
        return copy[node];
    }
};
