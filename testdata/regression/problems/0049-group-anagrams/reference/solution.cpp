class Solution {
   public:
    vector<vector<string>> groupAnagrams(vector<string>& strs) {
        map<string, vector<string>> groups;
        for (auto s : strs) {
            auto key = s;
            sort(key.begin(), key.end());
            groups[key].push_back(s);
        }
        vector<vector<string>> out;
        for (auto& [k, v] : groups) out.push_back(v);
        return out;
    }
};
