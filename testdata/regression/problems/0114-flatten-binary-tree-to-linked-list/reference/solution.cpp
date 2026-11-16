class Solution {
   public:
    void flatten(TreeNode* root) {
        for (auto n = root; n; n = n->right)
            if (n->left) {
                auto tail = n->left;
                while (tail->right) tail = tail->right;
                tail->right = n->right;
                n->right = n->left;
                n->left = nullptr;
            }
    }
};
