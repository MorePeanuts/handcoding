class Solution {
   public:
    void rotate(vector<int>& nums, int k) {
        if (nums.empty()) return;
        k %= nums.size();
        std::rotate(nums.begin(), nums.end() - k, nums.end());
    }
};
