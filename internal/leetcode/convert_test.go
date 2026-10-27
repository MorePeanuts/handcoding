package leetcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func loadQuestion(t *testing.T, slug string) *Question {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", slug+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var q Question
	if err := json.Unmarshal(data, &q); err != nil {
		t.Fatal(err)
	}
	return &q
}

// For judge-defined types the signature comes from the Go template, since
// metaData describes LeetCode's test data. The fixtures are LeetCode's
// questions with the statement cut down to the examples.
func TestConvertJudgeDefinedTypes(t *testing.T) {
	tests := []struct {
		slug, signature string
		examples        int // examples kept
		expected        int // of which with an expected output
	}{
		{"clone-graph", "cloneGraph(node GraphNode) GraphNode", 3, 3},
		{"copy-list-with-random-pointer", "copyRandomList(head RandomListNode) RandomListNode", 3, 3},
		{"n-ary-tree-preorder-traversal", "preorder(root NaryNode) integer[]", 2, 2},
		// LeetCode writes the levels as [1,#,2,3,#,4,5,7,#].
		{"populating-next-right-pointers-in-each-node-ii", "connect(root NextTreeNode) NextTreeNode", 2, 1},
		{"construct-quad-tree", "construct(grid integer[][]) QuadNode", 2, 2},
		// The Go template calls the parameter root; Python and the
		// statement call it head.
		{"flatten-a-multilevel-doubly-linked-list", "flatten(head MultilevelNode) MultilevelNode", 3, 3},
		{"mini-parser", "deserialize(s string) NestedInteger", 2, 2},
		{"employee-importance", "getImportance(employees Employee[], id integer) integer", 2, 2},
		// The examples give the constructor's input but no operations.
		{"flatten-nested-list-iterator", "NestedIterator(nestedList NestedInteger[]); next() integer; hasNext() boolean", 0, 0},
	}
	for _, tt := range tests {
		c, err := Convert(loadQuestion(t, tt.slug), SiteCom, false)
		if err != nil {
			t.Errorf("%s: %v", tt.slug, err)
			continue
		}
		if got := signature(c.Problem); got != tt.signature {
			t.Errorf("%s: signature %s, want %s", tt.slug, got, tt.signature)
		}
		withExpected := 0
		for _, cs := range c.Cases {
			if cs.HasExpected() {
				withExpected++
			}
		}
		if len(c.Cases) != tt.examples || withExpected != tt.expected {
			t.Errorf("%s: %d examples, %d with expected; want %d, %d (warnings %q)", tt.slug, len(c.Cases), withExpected, tt.examples, tt.expected, c.Warnings)
		}
	}
}

func signature(p *problem.Problem) string {
	params := func(ps []problem.Param) string {
		var parts []string
		for _, prm := range ps {
			parts = append(parts, prm.Name+" "+prm.Type)
		}
		return strings.Join(parts, ", ")
	}
	if p.Kind == problem.KindClass {
		parts := []string{fmt.Sprintf("%s(%s)", p.Class.Name, params(p.Class.Constructor))}
		for _, m := range p.Class.Methods {
			parts = append(parts, fmt.Sprintf("%s(%s) %s", m.Name, params(m.Params), m.Returns))
		}
		return strings.Join(parts, "; ")
	}
	return fmt.Sprintf("%s(%s) %s", p.Function.Name, params(p.Function.Params), p.Function.Returns)
}

// Templates using a Node hc does not know, or a type it cannot drive
// besides a judge-defined one, are rejected.
func TestConvertUnsupportedTemplates(t *testing.T) {
	tests := []struct {
		name, template, want string
	}{
		{"Node with a parent", "/**\n * type Node struct {\n *     Val int\n *     Left *Node\n *     Right *Node\n *     Parent *Node\n * }\n */\n\nfunc lowestCommonAncestor(p *Node, q *Node) *Node {\n\n}",
			"uses a Node with the fields Left, Parent, Right, Val"},
		{"other judge type", "/**\n * type Node struct {\n *     Val int\n *     Children []*Node\n * }\n */\n\nfunc f(root *Node, m *MountainArray) int {\n\n}",
			"uses the Go type *MountainArray"},
	}
	for _, tt := range tests {
		q := &Question{QuestionFrontendID: "1", TitleSlug: "x", MetaData: `{"name":"f","params":[{"name":"root","type":"integer"}],"return":{"type":"integer"},"manual":true}`,
			CodeSnippets: goSnippet(tt.template)}
		_, err := Convert(q, SiteCom, false)
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want ErrUnsupported with %q", tt.name, err, tt.want)
		}
	}
}

// Encoders judged by decoding are imported as round trips: the class from
// the templates (metaData names one function), encode and decode from the
// harness in their comments, examples as encode's parameter.
func TestConvertRoundTrip(t *testing.T) {
	tests := []struct {
		slug, signature, input, expected string
		rt                               problem.RoundTrip
	}{
		{"serialize-and-deserialize-binary-tree", "Codec(); serialize(root TreeNode) string; deserialize(data string) TreeNode",
			`{"root":[1,2,3,null,null,4,5]}`, `[1,2,3,null,null,4,5]`, problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: problem.InstancesSeparate}},
		// The harness keeps the encoded tree in a variable.
		{"serialize-and-deserialize-bst", "Codec(); serialize(root TreeNode) string; deserialize(data string) TreeNode",
			`{"root":[2,1,3]}`, `[2,1,3]`, problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: problem.InstancesSeparate}},
		{"encode-and-decode-tinyurl", "Codec(); encode(longUrl string) string; decode(shortUrl string) string",
			`{"longUrl":"https://leetcode.com/problems/design-tinyurl"}`, `"https://leetcode.com/problems/design-tinyurl"`, problem.RoundTrip{Encode: "encode", Decode: "decode", Instances: problem.InstancesSame}},
	}
	for _, tt := range tests {
		c, err := Convert(loadQuestion(t, tt.slug), SiteCom, false)
		if err != nil {
			t.Errorf("%s: %v", tt.slug, err)
			continue
		}
		p := c.Problem
		if got := signature(p); got != tt.signature {
			t.Errorf("%s: signature %s, want %s", tt.slug, got, tt.signature)
		}
		if p.Judge.RoundTrip == nil || *p.Judge.RoundTrip != tt.rt {
			t.Errorf("%s: round trip %+v, want %+v", tt.slug, p.Judge.RoundTrip, tt.rt)
		}
		if p.Ready() {
			t.Errorf("%s: judge.compare %q, want it left for hc gen", tt.slug, p.Judge.Compare)
		}
		if len(c.Cases) == 0 {
			t.Errorf("%s: no examples (warnings %q)", tt.slug, c.Warnings)
			continue
		}
		in, _ := json.Marshal(c.Cases[0].Input)
		if string(in) != tt.input || string(c.Cases[0].Expected) != tt.expected {
			t.Errorf("%s: first example %s -> %s, want %s -> %s", tt.slug, in, c.Cases[0].Expected, tt.input, tt.expected)
		}
		for _, w := range c.Warnings {
			if strings.Contains(w, "own checker") {
				t.Errorf("%s: warning %q", tt.slug, w)
			}
		}
	}
}

func TestHarnessRoundTrip(t *testing.T) {
	tests := []struct {
		comment string
		want    *problem.RoundTrip
	}{
		{"ser = Codec()\ndeser = Codec()\nans = deser.deserialize(ser.serialize(root))",
			&problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: problem.InstancesSeparate}},
		{"codec = Codec()\ncodec.decode(codec.encode(url))",
			&problem.RoundTrip{Encode: "encode", Decode: "decode", Instances: problem.InstancesSame}},
		{"ser := Constructor();\ndeser := Constructor();\ndata := ser.serialize(root);\nans := deser.deserialize(data);",
			&problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: problem.InstancesSeparate}},
		{"obj := Constructor();\nurl := obj.encode(longUrl);\nans := obj.decode(url);",
			&problem.RoundTrip{Encode: "encode", Decode: "decode", Instances: problem.InstancesSame}},
		// A design problem's calls do not feed each other.
		{"obj := Constructor(capacity);\nparam_1 := obj.Get(key);\nobj.Put(key,value);", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := harnessRoundTrip(tt.comment)
		if (got == nil) != (tt.want == nil) || got != nil && *got != *tt.want {
			t.Errorf("%q: %+v, want %+v", tt.comment, got, tt.want)
		}
	}
}

// A harness over a function template cannot be driven as a round trip.
func TestConvertRoundTripFunction(t *testing.T) {
	q := &Question{QuestionFrontendID: "1", Title: "X", TitleSlug: "x", MetaData: `{"name":"f","params":[{"name":"s","type":"string"}],"return":{"type":"string"},"manual":true}`,
		CodeSnippets: goSnippet("func f(s string) string {\n\n}\n\n/**\n * data := c.encode(s)\n * ans := c.decode(data)\n */")}
	if _, err := Convert(q, SiteCom, false); err == nil || !strings.Contains(err.Error(), "judge.round_trip is for class problems only") {
		t.Errorf("error = %v", err)
	}
}
