package leetcode

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Captured from leetcode.cn on 2026-11-13: the API says 0,1,2 while
// the statement gives 10,0,1. Counts alone cannot establish correspondence.
func TestStatementExamplePairs(t *testing.T) {
	for _, inputs := range []string{"0\n1\n2", "0\n10\n1", "10\n0\n1", "", "not JSON", "10"} {
		t.Run(inputs, func(t *testing.T) {
			q := loadQuestion(t, "count-primes")
			q.ExampleTestcases = inputs
			c, err := Convert(q, SiteCN)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Cases) != 3 || len(c.Warnings) != 0 {
				t.Fatalf("cases=%+v warnings=%q", c.Cases, c.Warnings)
			}
			for i, n := range []string{"10", "0", "1"} {
				want := "0"
				if i == 0 {
					want = "4"
				}
				if string(c.Cases[i].Input["n"]) != n || string(c.Cases[i].Expected) != want {
					t.Errorf("case %d: %+v; want n=%s -> %s", i, c.Cases[i], n, want)
				}
			}
		})
	}
}

func TestUncertainStatementExamples(t *testing.T) {
	for _, tt := range []struct {
		name, content   string
		inputs, outputs []string
	}{
		{"missing input", "<pre><strong>Output:</strong> 4</pre>", []string{"0", "1", "2"}, []string{"", "", ""}},
		{"unparseable input", "<pre>Input: n = ...\nOutput: 4</pre>", []string{"0", "1", "2"}, []string{"", "", ""}},
		{"extra input", "<pre>Input: n = 10, extra = 1\nOutput: 4</pre>", []string{"0", "1", "2"}, []string{"", "", ""}},
		{"partial", "<pre>Input: n = 10\nOutput: 4</pre><pre>Input: n = ...\nOutput: 99</pre>", []string{"10", "0", "1", "2"}, []string{"4", "", "", ""}},
		{"no shifting", "<pre>Input: n = 0</pre><pre>Input: n = 10\nOutput: 4</pre>", []string{"0", "10"}, []string{"", "4"}},
		{"ambiguous block", "<pre>Input: n = 10\nOutput: 4\nInput: n = 0\nOutput: 0</pre>", []string{"0", "1", "2"}, []string{"", "", ""}},
		{"trailing prose", "<pre>Input: n = 10 (ten)\nOutput: 4</pre>", []string{"0", "1", "2"}, []string{"", "", ""}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := loadQuestion(t, "count-primes")
			q.Content = tt.content
			c, err := Convert(q, SiteCN)
			if err != nil {
				t.Fatal(err)
			}
			var inputs, outputs []string
			for _, cs := range c.Cases {
				inputs = append(inputs, string(cs.Input["n"]))
				outputs = append(outputs, string(cs.Expected))
			}
			if !reflect.DeepEqual(inputs, tt.inputs) || !reflect.DeepEqual(outputs, tt.outputs) || len(c.Warnings) == 0 {
				t.Fatalf("inputs=%v outputs=%v warnings=%q", inputs, outputs, c.Warnings)
			}
		})
	}
}

func TestStatementInputSyntax(t *testing.T) {
	for _, tt := range []struct {
		input       string
		names, keys []string
		want        string
	}{
		{`b = [[1, 2], [3]], a = "x, b = \"y\""`, []string{"a", "b"}, []string{"a", "b"}, `{"a":"x, b = \"y\"","b":[[1,2],[3]]}`},
		{"[\"Cache\", \"get\"]\n[[2], [1]]", []string{"ops", "args"}, []string{"ops", "args"}, `{"args":[[2],[1]],"ops":["Cache","get"]}`},
		{"skip = 5, list = [1, 2]", []string{"skip", "list"}, []string{"", "head"}, `{"head":[1,2]}`},
		{"n = 9007199254740993", []string{"n"}, []string{"n"}, `{"n":9007199254740993}`},
		{"n = 1, n = 2", []string{"n", "m"}, []string{"n", "m"}, ""},
		{"n = 1,", []string{"n"}, []string{"n"}, ""},
		{"n = 1true", []string{"n", "m"}, []string{"n", "m"}, ""},
		{"unknown = 1, m = 2", []string{"n", "m"}, []string{"n", "m"}, ""},
		{"n = 1, 2", []string{"n", "m"}, []string{"n", "m"}, ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := statementInput(tt.input, tt.names, tt.keys)
			if tt.want == "" {
				if ok {
					t.Fatalf("accepted ambiguous input: %v", got)
				}
				return
			}
			b, _ := json.Marshal(got)
			if !ok || string(b) != tt.want {
				t.Fatalf("%s, %v; want %s", b, ok, tt.want)
			}
		})
	}
}

func TestStatementExampleHTML(t *testing.T) {
	for _, content := range []string{
		`<pre><strong>Input:</strong> n = 10<strong>Output:</strong> 4<strong>Explanation:</strong> four primes</pre>`,
		`<div class="example-block"><p><strong>Input:</strong> <span>n = 10</span></p><p><strong>Output:</strong> <span>4</span></p></div>`,
		`<pre><b>输入：</b>n&nbsp;=&nbsp;10<br><b>输出：</b>4<br><b>解释：</b>四个质数</pre>`,
	} {
		q := loadQuestion(t, "count-primes")
		q.Content = content
		if strings.Contains(content, "输入") {
			q.Content = englishPlaceholder
			q.TranslatedContent = content
		}
		c, err := Convert(q, SiteCN)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Cases) != 1 || string(c.Cases[0].Input["n"]) != "10" || string(c.Cases[0].Expected) != "4" || len(c.Warnings) != 0 {
			t.Errorf("content=%s cases=%+v warnings=%q", content, c.Cases, c.Warnings)
		}
	}
}
