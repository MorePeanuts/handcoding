static int digits(long long value) {
    int sum = 0;
    while (value) {
        sum += (int)(value % 10);
        value /= 10;
    }
    return sum;
}
long long makeIntegerBeautiful(long long n, int target) {
    long long value = n, place = 10;
    while (digits(value) > target) {
        value = (value / place + 1) * place;
        place *= 10;
    }
    return value - n;
}
