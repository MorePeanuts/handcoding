char* longestPalindrome(char* s) {
    int n = (int)strlen(s), start = 0, length = 0;
    for (int i = 0; i < n; ++i) {
        for (int d = 0; d < 2; ++d) {
            int l = i, r = i + d;
            while (l >= 0 && r < n && s[l] == s[r]) {
                if (r - l + 1 > length) {
                    start = l;
                    length = r - l + 1;
                }
                --l;
                ++r;
            }
        }
    }
    char* out = malloc((size_t)length + 1);
    memcpy(out, s + start, (size_t)length);
    out[length] = '\0';
    return out;
}
