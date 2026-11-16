class Solution {
    vector<long long> prefix;
    mt19937 rng{random_device{}()};

   public:
    Solution(vector<int>& w) {
        long long sum = 0;
        for (int x : w) {
            sum += x;
            prefix.push_back(sum);
        }
    }
    int pickIndex() {
        long long x =
            uniform_int_distribution<long long>(1, prefix.back())(rng);
        return lower_bound(prefix.begin(), prefix.end(), x) - prefix.begin();
    }
};
