# Workspace-managed C libraries

Only `lock.json` is tracked and embedded. It pins official HTTPS source URLs and
SHA-256 digests. Setup downloads files into `.hc/c/<lock hash>` and verifies all
files before publishing. Existing complete installations work offline. Neither
third-party library source nor a dependency archive belongs in Git or hc's binary.

- [yyjson 0.10.0](https://github.com/ibireme/yyjson/tree/0.10.0): MIT, JSON codec
  implementation (`yyjson.c`, `yyjson.h`). License notices remain in the files.
- [uthash 2.3.0](https://github.com/troydhanson/uthash/tree/v2.3.0): revised BSD,
  native LeetCode hash-table macros (`uthash.h`, including its license notice).

GLib is not downloaded. The user supplies local development headers/libraries
and pkg-config; the driver obtains compiler/linker flags with `glib-2.0`.

To update a dependency, review its license/API and official release, update its
URL and digest, and rerun environment lifecycle, C11 and native-template tests.
The lock digest gives the new release its own workspace directory.
