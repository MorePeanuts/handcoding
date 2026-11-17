int* findOrder(int numCourses, int** prerequisites, int prerequisitesSize,
               int* prerequisitesColSize, int* returnSize) {
    (void)prerequisitesColSize;
    int* head = malloc((size_t)numCourses * sizeof(int));
    int* next = malloc((size_t)prerequisitesSize * sizeof(int));
    int* degree = calloc((size_t)numCourses, sizeof(int));
    int* order = malloc((size_t)numCourses * sizeof(int));
    for (int i = 0; i < numCourses; ++i) head[i] = -1;
    for (int i = 0; i < prerequisitesSize; ++i) {
        int from = prerequisites[i][1], to = prerequisites[i][0];
        next[i] = head[from];
        head[from] = i;
        ++degree[to];
    }
    int count = 0;
    for (int i = 0; i < numCourses; ++i)
        if (!degree[i]) order[count++] = i;
    for (int at = 0; at < count; ++at)
        for (int edge = head[order[at]]; edge != -1; edge = next[edge])
            if (!--degree[prerequisites[edge][0]])
                order[count++] = prerequisites[edge][0];
    *returnSize = count == numCourses ? count : 0;
    free(head);
    free(next);
    free(degree);
    return order;
}
