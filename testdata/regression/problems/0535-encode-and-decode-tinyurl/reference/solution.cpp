class Codec {
    unordered_map<string, string> urls;
    int serial = 0;

   public:
    string encode(string url) {
        string key = "https://hc/" + to_string(++serial);
        urls[key] = url;
        return key;
    }
    string decode(string key) { return urls.at(key); }
};
