class Solution {
   public:
    TreeNode* build(vector<int>& a, int l, int r) {
        if (l >= r) return nullptr;
        int m = (l + r) / 2;
        return new TreeNode(a[m], build(a, l, m), build(a, m + 1, r));
    }
    TreeNode* sortedArrayToBST(vector<int>& nums) {
        return build(nums, 0, nums.size());
    }
};
