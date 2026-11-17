class Solution {
   public:
    Node* copyRandomList(Node* head) {
        unordered_map<Node*, Node*> copy;
        for (auto n = head; n; n = n->next) copy[n] = new Node(n->val);
        for (auto n = head; n; n = n->next) {
            copy[n]->next = n->next ? copy[n->next] : nullptr;
            copy[n]->random = n->random ? copy[n->random] : nullptr;
        }
        return head ? copy[head] : nullptr;
    }
};
