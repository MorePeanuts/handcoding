class Solution {
   public:
    NestedInteger parse(const string& s, size_t& i) {
        if (s[i] != '[') {
            size_t end;
            int n = stoi(s.substr(i), &end);
            i += end;
            return NestedInteger(n);
        }
        ++i;
        NestedInteger out;
        while (s[i] != ']') {
            out.add(parse(s, i));
            if (s[i] == ',') ++i;
        }
        ++i;
        return out;
    }
    NestedInteger deserialize(string s) {
        size_t i = 0;
        return parse(s, i);
    }
};
