class Solution {
   public:
    int getImportance(vector<Employee*> employees, int id) {
        unordered_map<int, Employee*> byid;
        for (auto e : employees) byid[e->id] = e;
        vector<int> todo{id};
        int sum = 0;
        while (!todo.empty()) {
            auto e = byid.at(todo.back());
            todo.pop_back();
            sum += e->importance;
            for (int x : e->subordinates) todo.push_back(x);
        }
        return sum;
    }
};
