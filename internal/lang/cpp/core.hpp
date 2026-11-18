#pragma once
#include <atomic>
#include <chrono>
#include <cstdlib>
#include <fstream>
#include <functional>
#include <iostream>
#include <mutex>
#include <thread>
#include <nlohmann/json.hpp>
using namespace std;
namespace hc {
using J = nlohmann::json;
inline void check(bool ok, const char* message) {
    if (!ok) throw runtime_error(message);
}
// A sequence lock gives the watchdog a consistent clock snapshot without a
// mutex around each method call of a large class case.
extern atomic<unsigned long long> sequence;
extern atomic<long long> started, elapsed;
inline long long now() {
    return chrono::duration_cast<chrono::nanoseconds>(
               chrono::steady_clock::now().time_since_epoch())
        .count();
}
inline void start() {
    sequence.fetch_add(1);
    started.store(now());
    sequence.fetch_add(1);
}
inline void stop() {
    long long end = now();
    sequence.fetch_add(1);
    long long begin = started.exchange(0);
    if (begin) elapsed.fetch_add(end - begin);
    sequence.fetch_add(1);
}
inline long long spent() {
    for (;;) {
        auto seq = sequence.load();
        if (seq & 1) continue;
        auto e = elapsed.load(), s = started.load();
        if (seq == sequence.load()) return e + (s ? now() - s : 0);
    }
}
template <class F>
auto timed(F&& f) {
    start();
    try {
        if constexpr (is_void_v<invoke_result_t<F>>) {
            f();
            stop();
        } else {
            auto out = f();
            stop();
            return out;
        }
    } catch (...) {
        stop();
        throw;
    }
}
J call(const J&);
int run();
}  // namespace hc
