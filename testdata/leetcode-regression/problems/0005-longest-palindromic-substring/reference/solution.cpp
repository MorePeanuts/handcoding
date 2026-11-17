class Solution {
   public:
    string longestPalindrome(string s) {
        int start = 0, length = 0;
        for (int i = 0; i < (int)s.size(); ++i)
            for (int d = 0; d < 2; ++d) {
                int l = i, r = i + d;
                while (l >= 0 && r < (int)s.size() && s[l] == s[r]) {
                    if (r - l + 1 > length) {
                        start = l;
                        length = r - l + 1;
                    }
                    --l;
                    ++r;
                }
            }
        return s.substr(start, length);
    }
};
