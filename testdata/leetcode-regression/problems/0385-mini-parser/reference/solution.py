from hc_runtime import NestedInteger


class Solution:
    def deserialize(self, s: str) -> NestedInteger:
        if s[0] != "[":
            return NestedInteger(int(s))
        stack: list[NestedInteger] = []
        num = ""
        for ch in s:
            if ch == "[":
                stack.append(NestedInteger())
            elif ch in ",]":
                if num:
                    stack[-1].add(NestedInteger(int(num)))
                    num = ""
                if ch == "]" and len(stack) > 1:
                    inner = stack.pop()
                    stack[-1].add(inner)
            else:
                num += ch
        return stack[0]
