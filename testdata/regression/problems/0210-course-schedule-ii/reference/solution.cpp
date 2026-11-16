class Solution {
   public:
    vector<int> findOrder(int n, vector<vector<int>>& prerequisites) {
        vector<vector<int>> edges(n);
        vector<int> degree(n), out;
        for (auto e : prerequisites) {
            edges[e[1]].push_back(e[0]);
            ++degree[e[0]];
        }
        queue<int> q;
        for (int i = 0; i < n; ++i)
            if (!degree[i]) q.push(i);
        while (!q.empty()) {
            int x = q.front();
            q.pop();
            out.push_back(x);
            for (int y : edges[x])
                if (!--degree[y]) q.push(y);
        }
        return (int)out.size() == n ? out : vector<int>{};
    }
};
