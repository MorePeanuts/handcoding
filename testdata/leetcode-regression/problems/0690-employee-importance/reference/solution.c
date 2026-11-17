typedef struct Entry {
    int id;
    struct Employee* employee;
    UT_hash_handle hh;
} Entry;
int getImportance(struct Employee** employees, int employeesSize, int id) {
    Entry *entries = NULL, *entry, *tmp;
    for (int i = 0; i < employeesSize; ++i) {
        entry = malloc(sizeof(*entry));
        entry->id = employees[i]->id;
        entry->employee = employees[i];
        HASH_ADD_INT(entries, id, entry);
    }
    int* todo = malloc((size_t)employeesSize * sizeof(int));
    int size = 0, result = 0;
    todo[size++] = id;
    while (size) {
        int key = todo[--size];
        HASH_FIND_INT(entries, &key, entry);
        result += entry->employee->importance;
        for (int i = 0; i < entry->employee->subordinatesSize; ++i)
            todo[size++] = entry->employee->subordinates[i];
    }
    HASH_ITER(hh, entries, entry, tmp) {
        HASH_DEL(entries, entry);
        free(entry);
    }
    free(todo);
    return result;
}
