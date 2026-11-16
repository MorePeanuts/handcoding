class Solution {
   public:
    Node* flatten(Node* head) {
        vector<Node*> todo;
        if (head) todo.push_back(head);
        Node* prev = nullptr;
        while (!todo.empty()) {
            auto n = todo.back();
            todo.pop_back();
            if (n->next) todo.push_back(n->next);
            if (n->child) todo.push_back(n->child);
            n->child = nullptr;
            n->prev = prev;
            if (prev) prev->next = n;
            prev = n;
        }
        if (prev) prev->next = nullptr;
        return head;
    }
};
