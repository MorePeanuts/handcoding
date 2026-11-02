package python

import (
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestLeetCode(t *testing.T) {
	fn := &problem.Problem{Slug: "two-sum", Kind: problem.KindFunction, Function: &problem.Function{Name: "twoSum"}}
	src := `from typing import Dict, List, Optional, Set, Tuple  # noqa: F401

from hc_runtime import ListNode, TreeNode  # noqa: F401
from hc_runtime import GraphNode as Node  # noqa: F401
from hc_runtime import (
    NestedInteger,
    Employee,
)
import hc_runtime
from hc_runtime import \
    decode


class Solution:
    def twoSum(self, nums: List[int], target: int) -> List[int]:
        return []  # hc_runtime is not needed here
`
	slug, code, err := Language{}.LeetCode(fn, src)
	if err != nil {
		t.Fatal(err)
	}
	want := `from typing import Dict, List, Optional, Set, Tuple  # noqa: F401



class Solution:
    def twoSum(self, nums: List[int], target: int) -> List[int]:
        return []  # hc_runtime is not needed here
`
	if slug != "python3" || code != want {
		t.Errorf("LeetCode = %q,\n%s\nwant\n%s", slug, code, want)
	}

	// A module-level function runs in hc but not on LeetCode.
	if _, _, err := (Language{}).LeetCode(fn, "def twoSum(nums, target):\n    return []\n"); err == nil || !strings.Contains(err.Error(), "class Solution") {
		t.Errorf("module-level function: err = %v", err)
	}
	// A class problem has no Solution class.
	cls := &problem.Problem{Slug: "lru-cache", Kind: problem.KindClass, Class: &problem.Class{Name: "LRUCache"}}
	if _, code, err := (Language{}).LeetCode(cls, "class LRUCache:\n    pass\n"); err != nil || code != "class LRUCache:\n    pass\n" {
		t.Errorf("class problem: %q, %v", code, err)
	}
	// hc_runtime used beyond its imports does not exist on LeetCode.
	if _, _, err := (Language{}).LeetCode(fn, "import hc_runtime\nclass Solution:\n    x = hc_runtime.decode\n"); err == nil || !strings.Contains(err.Error(), "hc_runtime") {
		t.Errorf("hc_runtime use: err = %v", err)
	}
}
