package leetcode

import (
	"errors"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func TestConvertFunction(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "1",
		Title:              "Two Sum",
		TitleSlug:          "two-sum",
		Difficulty:         "Easy",
		Content: `<p>Given an array... You can return the answer in any order.</p>
<p><strong class="example">Example 1:</strong></p>
<pre><strong>Input:</strong> nums = [2,7,11,15], target = 9
<strong>Output:</strong> [0,1]
<strong>Explanation:</strong> Because nums[0] + nums[1] == 9.
</pre>
<div class="example-block"><p><strong>Input:</strong> <span class="example-io">nums = [3,2,4], target = 6</span></p>
<p><strong>Output:</strong> <span class="example-io">[1,2]</span></p></div>
<p><code>2 &lt;= nums.length &lt;= 10<sup>4</sup></code></p>`,
		MetaData:         `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]","size":2}}`,
		ExampleTestcases: "[2,7,11,15]\n9\n[3,2,4]\n6",
	}
	q.CodeSnippets = append(q.CodeSnippets, struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	}{"golang", "func twoSum(nums []int, target int) []int {\n\n}"})

	c, err := Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	p := c.Problem
	// The judge rule is left for hc gen even though the statement says
	// "in any order".
	if p.Kind != problem.KindFunction || p.Function.Name != "twoSum" || p.Ready() {
		t.Fatalf("unexpected problem: %+v", p)
	}
	if len(c.Cases) != 2 {
		t.Fatalf("got %d cases", len(c.Cases))
	}
	if string(c.Cases[0].Expected) != "[0,1]" || string(c.Cases[1].Expected) != "[1,2]" {
		t.Fatalf("expected outputs: %s %s", c.Cases[0].Expected, c.Cases[1].Expected)
	}
	if string(c.Cases[1].Input["target"]) != "6" {
		t.Fatalf("input: %s", c.Cases[1].Input["target"])
	}
	if !strings.Contains(c.Description, "10^4") {
		t.Errorf("superscript lost in description:\n%s", c.Description)
	}
	if _, ok := c.Templates["solution.go"]; !ok {
		t.Error("go template missing")
	}
}

func TestConvertClassAndInPlace(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "146", Title: "LRU Cache", TitleSlug: "lru-cache", Difficulty: "Medium",
		Content: "<pre><strong>Input</strong>\n[...]\n<strong>Output</strong>\n[null, -1]\n</pre>",
		MetaData: `{"classname":"LRUCache","constructor":{"params":[{"type":"integer","name":"capacity"}]},
			"methods":[{"params":[{"type":"integer","name":"key"}],"name":"get","return":{"type":"integer"}}],
			"return":{"type":"boolean"},"systemdesign":true}`,
		ExampleTestcases: `["LRUCache","get"]` + "\n" + `[[2],[1]]`,
	}
	c, err := Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Problem.Kind != problem.KindClass || len(c.Cases) != 1 || string(c.Cases[0].Expected) != "[null,-1]" {
		t.Fatalf("class: %+v %+v", c.Problem, c.Cases)
	}

	q = &Question{
		QuestionFrontendID: "189", Title: "Rotate Array", TitleSlug: "rotate-array", Difficulty: "Medium",
		MetaData: `{"name":"rotate","params":[{"name":"nums","type":"integer[]"},{"name":"k","type":"integer"}],"return":{"type":"void"},"output":{"paramindex":0}}`,
	}
	c, err = Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Problem.Judge.Output != "nums" {
		t.Fatalf("in-place output: %q", c.Problem.Judge.Output)
	}
}

// Parameters of one language's template only (C passes array lengths) are
// not part of the signature, so the examples still match it.
func TestConvertSkipsLanguageOnlyParams(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "703", Title: "Kth Largest Element in a Stream", TitleSlug: "kth-largest-element-in-a-stream", Difficulty: "Easy",
		Content: "<pre><strong>Input</strong>\n[...]\n<strong>Output</strong>\n[null, 4]\n</pre>",
		MetaData: `{"classname":"KthLargest","constructor":{"params":[{"type":"integer","name":"k"},{"type":"integer[]","name":"nums"},
			{"type":"integer","name":"numsSize","lang":"c","value":"size_2"}]},
			"methods":[{"params":[{"type":"integer","name":"val"},{"type":"integer","name":"valSize","lang":"c"}],"name":"add","return":{"type":"integer"}}],
			"return":{"type":"boolean"},"systemdesign":true}`,
		ExampleTestcases: `["KthLargest","add"]` + "\n" + `[[3,[4,5,8,2]],[3]]`,
	}
	c, err := Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	cl := c.Problem.Class
	if len(cl.Constructor) != 2 || cl.Constructor[1].Name != "nums" || len(cl.Methods[0].Params) != 1 {
		t.Fatalf("signature: %+v", cl)
	}
	if len(c.Cases) != 1 || len(c.Warnings) != 0 {
		t.Fatalf("cases %+v, warnings %q", c.Cases, c.Warnings)
	}

	q = &Question{
		QuestionFrontendID: "1", Title: "Sum", TitleSlug: "sum", Difficulty: "Easy",
		MetaData: `{"name":"sum","params":[{"name":"nums","type":"integer[]"},{"name":"numsSize","type":"integer","lang":"c"}],"return":{"type":"integer"}}`,
	}
	c, err = Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	if ps := c.Problem.Function.Params; len(ps) != 1 || ps[0].Name != "nums" {
		t.Fatalf("function params: %+v", ps)
	}
}

func goSnippet(code string) []struct {
	LangSlug string `json:"langSlug"`
	Code     string `json:"code"`
} {
	return []struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	}{{"golang", code}}
}

func TestConvertUnsupported(t *testing.T) {
	tests := []struct {
		name string
		q    *Question
		want string
	}{
		{"unsupported type", &Question{
			QuestionFrontendID: "1095", TitleSlug: "find-in-mountain-array",
			MetaData: `{"name":"findInMountainArray","params":[{"name":"target","type":"integer"},{"name":"mountainArr","type":"MountainArray"}],"return":{"type":"integer"},"manual":true}`,
		}, "uses the type MountainArray"},
		{"no name", &Question{
			QuestionFrontendID: "341", TitleSlug: "flatten-nested-list-iterator",
			MetaData: `{"name":"","params":[{"name":"nestedList","type":"list<NestedInteger>"}],"return":{"type":"integer"},"manual":true}`,
		}, "names no function or class"},
		// LeetCode's metadata for Clone Graph describes its own test data,
		// not the function the user writes.
		{"custom judge, different signature", &Question{
			QuestionFrontendID: "133", TitleSlug: "clone-graph",
			MetaData:     `{"name":"cloneGraph","params":[{"name":"edges","type":"integer[][]"}],"return":{"type":"boolean"},"manual":true}`,
			CodeSnippets: goSnippet("/**\n * type Node struct {...}\n */\n\nfunc cloneGraph(node *Node) *Node {\n\n}"),
		}, "real signature cloneGraph(node *Node)"},
		{"custom judge, fewer parameters", &Question{
			QuestionFrontendID: "237", TitleSlug: "delete-node-in-a-linked-list",
			MetaData:     `{"name":"deleteNode","params":[{"name":"head","type":"ListNode"},{"name":"node","type":"integer"}],"return":{"type":"void"},"manual":true}`,
			CodeSnippets: goSnippet("func deleteNode(node *ListNode) {\n\n}"),
		}, "real signature deleteNode(node *ListNode)"},
	}
	for _, tt := range tests {
		_, err := Convert(tt.q, SiteCom, false)
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want ErrUnsupported with %q", tt.name, err, tt.want)
		}
	}
}

func TestConvertOutputLength(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "26", Title: "Remove Duplicates", TitleSlug: "remove-duplicates-from-sorted-array",
		MetaData:         `{"name":"removeDuplicates","params":[{"name":"nums","type":"integer[]"}],"return":{"type":"integer"},"output":{"paramindex":0,"size":"ret"}}`,
		Content:          "<pre><strong>Output:</strong> 2, nums = [1,2,_]\n</pre>",
		ExampleTestcases: "[1,1,2]",
	}
	c, err := Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	if j := c.Problem.Judge; j.Output != "nums" || j.OutputLength != problem.OutputLengthReturn {
		t.Fatalf("judge = %+v", j)
	}
}

func TestConvertChineseOnly(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "LCR 141", Title: "训练计划 III", TitleSlug: "fan-zhuan-lian-biao-lcof",
		Content:           "<p>English description is not available for the problem. Please switch to Chinese.</p>",
		TranslatedContent: "<p>反转链表。</p><pre><strong>输入：</strong>head = [1,2]\n<strong>输出：</strong>[2,1]\n</pre>",
		MetaData:          `{"name":"trainningPlan","params":[{"name":"head","type":"ListNode"}],"return":{"type":"ListNode"}}`,
		ExampleTestcases:  "[1,2]",
	}
	c, err := Convert(q, SiteCN, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Description, "反转链表") {
		t.Errorf("description should fall back to Chinese:\n%s", c.Description)
	}
	if len(c.Cases) != 1 || string(c.Cases[0].Expected) != "[2,1]" {
		t.Errorf("cases = %+v", c.Cases)
	}
}

func TestConvertDropsMismatchedExpected(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "70", Title: "Climbing Stairs", TitleSlug: "climbing-stairs",
		MetaData:         `{"name":"climbStairs","params":[{"name":"n","type":"integer"}],"return":{"type":"integer"}}`,
		Content:          "<pre><strong>Output:</strong> [2]\n</pre>",
		ExampleTestcases: "2",
	}
	c, err := Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Cases) != 1 || c.Cases[0].HasExpected() || len(c.Warnings) == 0 {
		t.Errorf("cases = %+v, warnings = %q", c.Cases, c.Warnings)
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		ref, site, slug string
	}{
		{"https://leetcode.com/problems/two-sum/", "com", "two-sum"},
		{"https://leetcode.com/problems/two-sum/description/", "com", "two-sum"},
		{"https://leetcode.com/problems/two-sum/description/?envType=study-plan-v2&envId=top-interview-150", "com", "two-sum"},
		{"leetcode.cn/problems/two-sum", "cn", "two-sum"},
		{"https://leetcode.cn/problems/xoh6Oh/solutions/", "cn", "xoh6Oh"},
		{"https://www.leetcode.com/problems/two-sum#x", "com", "two-sum"},
	}
	for _, tt := range tests {
		site, slug, ok := ParseURL(tt.ref)
		if !ok || site.Name != tt.site || slug != tt.slug {
			t.Errorf("ParseURL(%q) = %s, %q, %v", tt.ref, site.Name, slug, ok)
		}
	}
	for _, ref := range []string{"two-sum", "146", "https://example.com/problems/x", "https://leetcode.com/contest/"} {
		if _, _, ok := ParseURL(ref); ok {
			t.Errorf("ParseURL(%q) should not match", ref)
		}
	}
	for ref, want := range map[string]bool{"146": true, "LCR 001": true, "lcp 01": true, "two-sum": false, "3sum": false} {
		if idRE.MatchString(ref) != want {
			t.Errorf("idRE(%q) = %v", ref, !want)
		}
	}
}
