class Solution {
   public:
    ListNode* mergeKLists(vector<ListNode*>& lists) {
        auto cmp = [](ListNode* a, ListNode* b) { return a->val > b->val; };
        priority_queue<ListNode*, vector<ListNode*>, decltype(cmp)> q(cmp);
        for (auto n : lists)
            if (n) q.push(n);
        ListNode dummy;
        auto tail = &dummy;
        while (!q.empty()) {
            auto n = q.top();
            q.pop();
            if (n->next) q.push(n->next);
            tail->next = n;
            tail = n;
        }
        tail->next = nullptr;
        return dummy.next;
    }
};
