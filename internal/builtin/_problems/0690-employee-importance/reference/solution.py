from hc_runtime import Employee


class Solution:
    def getImportance(self, employees: list[Employee], id: int) -> int:
        by_id = {e.id: e for e in employees}
        total, stack = 0, [id]
        while stack:
            e = by_id[stack.pop()]
            total += e.importance
            stack.extend(e.subordinates)
        return total
