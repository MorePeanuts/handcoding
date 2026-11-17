typedef struct URL {
    char key[40];
    char* value;
    UT_hash_handle hh;
} URL;
static URL* urls;
static unsigned long serial;
char* encode(char* longUrl) {
    URL* entry = malloc(sizeof(*entry));
    snprintf(entry->key, sizeof(entry->key), "https://hc/%lu", ++serial);
    entry->value = malloc(strlen(longUrl) + 1);
    strcpy(entry->value, longUrl);
    HASH_ADD_STR(urls, key, entry);
    return entry->key;
}
char* decode(char* shortUrl) {
    URL* entry;
    HASH_FIND_STR(urls, shortUrl, entry);
    return entry ? entry->value : "";
}
