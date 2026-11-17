class Solution {
   public:
    void wiggleSort(vector<int>& nums) {
        auto a = nums;
        sort(a.begin(), a.end());
        int l = (a.size() + 1) / 2 - 1, r = a.size() - 1;
        for (size_t i = 0; i < nums.size(); ++i)
            nums[i] = i % 2 ? a[r--] : a[l--];
    }
};
