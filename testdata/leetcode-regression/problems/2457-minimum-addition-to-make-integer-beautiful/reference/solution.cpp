class Solution {
   public:
    long long makeIntegerBeautiful(long long n, int target) {
        auto sum = [](long long x) {
            int s = 0;
            while (x) {
                s += x % 10;
                x /= 10;
            }
            return s;
        };
        long long value = n, place = 10;
        while (sum(value) > target) {
            value = (value / place + 1) * place;
            place *= 10;
        }
        return value - n;
    }
};
