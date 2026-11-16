class Solution {
   public:
    void reorderList(ListNode* head) {
        vector<ListNode*> v;
        for (auto n = head; n; n = n->next) v.push_back(n);
        if (v.empty()) return;
        size_t l = 0, r = v.size() - 1;
        auto tail = v[l++];
        while (l <= r) {
            tail->next = v[r--];
            tail = tail->next;
            if (l <= r) {
                tail->next = v[l++];
                tail = tail->next;
            }
        }
        tail->next = nullptr;
    }
};
