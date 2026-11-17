double* convertTemperature(double celsius, int* returnSize) {
    double* out = malloc(2 * sizeof(double));
    *returnSize = 2;
    out[0] = celsius + 273.15;
    out[1] = celsius * 1.80 + 32.00;
    return out;
}
