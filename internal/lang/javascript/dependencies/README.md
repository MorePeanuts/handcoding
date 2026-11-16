# Workspace JavaScript dependencies

Only these manifests are tracked and embedded in hc. The package lock fixes exact versions, transitive dependencies and npm integrity hashes for the LeetCode environment. Package contents and licenses are downloaded by local npm into the user's `.hc/node/<lock hash>` during environment preparation, never into the repository or binary. Node, npm and TypeScript remain user-provided tools.

Source for LeetCode library versions: https://support.leetcode.com/hc/en-us/articles/360011833974-What-are-the-environments-for-the-programming-languages

To update, copy both JSON files into a scratch directory, deliberately change versions and update the lockfile there. Review and copy back only the manifests. Never commit node_modules or a dependency archive. Validate both Node 22 / TypeScript 5.7 and current supported versions, including repair and offline reuse. `hc env sync --lang javascript` repairs a workspace's environment; ordinary cache cleanup preserves it.
