class Solution {
   public:
    vector<ListNode*> splitListToParts(ListNode* head, int k) {
        int n = 0;
        for (auto p = head; p; p = p->next) ++n;
        vector<ListNode*> out;
        for (int i = 0; i < k; ++i) {
            int size = n / k + (i < n % k);
            out.push_back(head);
            for (int j = 1; j < size; ++j) head = head->next;
            if (head) {
                auto next = head->next;
                head->next = nullptr;
                head = next;
            }
        }
        return out;
    }
};
