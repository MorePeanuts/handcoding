# JSON for Modern C++

hc downloads the unmodified single-header distribution of nlohmann/json v3.12.0:
https://github.com/nlohmann/json/releases/tag/v3.12.0

Source: https://raw.githubusercontent.com/nlohmann/json/v3.12.0/single_include/nlohmann/json.hpp

SHA-256: `aaf127c04cb31c406e5b04a63f1ae89369fccde6d8fa7cdda1ed4f32dfc5de63`

MIT license: `LICENSE.MIT`. Only metadata and this license are tracked; the
header is neither tracked nor embedded. `environment.go` pins the version,
URL and digest. Setup downloads it into `.hc/cpp/json-<version>-<digest>/include/nlohmann/json.hpp`
using Go's HTTP client (no package manager). A cross-process lock serializes
installs and the verified header is published atomically. Ready environments
are checksum-verified and reused offline. `hc env sync -l cpp` repairs missing
or corrupt files; doctor is read-only and `hc clean` preserves this environment.

To update the dependency, verify the upstream license and change the pinned
version, URL and SHA-256 together. Older versions remain available for existing
hc processes. Run the C++17 and default-standard regression tests.
