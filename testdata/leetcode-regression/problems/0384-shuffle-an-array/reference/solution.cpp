class Solution {
    vector<int> original;
    mt19937 rng{random_device{}()};

   public:
    Solution(vector<int>& nums) : original(nums) {}
    vector<int> reset() { return original; }
    vector<int> shuffle() {
        auto out = original;
        std::shuffle(out.begin(), out.end(), rng);
        return out;
    }
};
