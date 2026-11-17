class Solution {
   public:
    double findMedianSortedArrays(vector<int>& a, vector<int>& b) {
        vector<int> v;
        merge(a.begin(), a.end(), b.begin(), b.end(), back_inserter(v));
        int n = v.size();
        return n % 2 ? v[n / 2]
                     : (static_cast<double>(v[n / 2 - 1]) + v[n / 2]) / 2;
    }
};
