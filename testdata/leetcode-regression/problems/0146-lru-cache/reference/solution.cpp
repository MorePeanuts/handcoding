class LRUCache {
    int capacity;
    list<pair<int, int>> order;
    unordered_map<int, list<pair<int, int>>::iterator> entries;

   public:
    LRUCache(int n) : capacity(n) {}
    int get(int key) {
        auto it = entries.find(key);
        if (it == entries.end()) return -1;
        order.splice(order.begin(), order, it->second);
        return it->second->second;
    }
    void put(int key, int value) {
        auto it = entries.find(key);
        if (it != entries.end()) {
            it->second->second = value;
            order.splice(order.begin(), order, it->second);
            return;
        }
        order.push_front({key, value});
        entries[key] = order.begin();
        if ((int)order.size() > capacity) {
            entries.erase(order.back().first);
            order.pop_back();
        }
    }
};
