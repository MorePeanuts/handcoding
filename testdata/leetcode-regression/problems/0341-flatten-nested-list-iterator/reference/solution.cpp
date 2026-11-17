class NestedIterator {
    vector<int> values;
    size_t at = 0;
    void flatten(const NestedInteger& n) {
        if (n.isInteger())
            values.push_back(n.getInteger());
        else
            for (auto& c : n.getList()) flatten(c);
    }

   public:
    NestedIterator(vector<NestedInteger>& nestedList) {
        for (auto& n : nestedList) flatten(n);
    }
    int next() { return values.at(at++); }
    bool hasNext() { return at < values.size(); }
};
