package leetcode

import (
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
	if p.Kind != problem.KindFunction || p.Function.Name != "twoSum" || p.Judge.Compare != problem.CompareUnordered {
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
		Content: "<pre><strong>Input</strong>\n[...]\n<strong>Output</strong>\n[null, null, 1]\n</pre>",
		MetaData: `{"classname":"LRUCache","constructor":{"params":[{"type":"integer","name":"capacity"}]},
			"methods":[{"params":[{"type":"integer","name":"key"}],"name":"get","return":{"type":"integer"}}],
			"return":{"type":"boolean"},"systemdesign":true}`,
		ExampleTestcases: `["LRUCache","get"]` + "\n" + `[[2],[1]]`,
	}
	c, err := Convert(q, SiteCom, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Problem.Kind != problem.KindClass || len(c.Cases) != 1 || string(c.Cases[0].Expected) != "[null,null,1]" {
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

func TestConvertUnsupported(t *testing.T) {
	q := &Question{
		QuestionFrontendID: "133", Title: "Clone Graph", TitleSlug: "clone-graph",
		MetaData: `{"name":"cloneGraph","params":[{"name":"node","type":"Node"}],"return":{"type":"Node"}}`,
	}
	if _, err := Convert(q, SiteCom, false); err == nil {
		t.Fatal("expected an error for unsupported types")
	}
}
