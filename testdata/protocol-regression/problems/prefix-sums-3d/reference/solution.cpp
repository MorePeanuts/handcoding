class Solution {
   public:
    vector<vector<vector<int>>> prefixSums(vector<vector<vector<int>>>& grid) {
        auto out = grid;
        for (size_t i = 0; i < out.size(); ++i)
            for (size_t j = 0; j < out[i].size(); ++j)
                for (size_t k = 0; k < out[i][j].size(); ++k) {
                    if (i) out[i][j][k] += out[i - 1][j][k];
                    if (j) out[i][j][k] += out[i][j - 1][k];
                    if (k) out[i][j][k] += out[i][j][k - 1];
                    if (i && j) out[i][j][k] -= out[i - 1][j - 1][k];
                    if (i && k) out[i][j][k] -= out[i - 1][j][k - 1];
                    if (j && k) out[i][j][k] -= out[i][j - 1][k - 1];
                    if (i && j && k) out[i][j][k] += out[i - 1][j - 1][k - 1];
                }
        return out;
    }
};
