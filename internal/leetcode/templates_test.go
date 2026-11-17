package leetcode

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func templateQuestion() *Question {
	q := &Question{QuestionFrontendID: "1", TitleSlug: "two-sum", Title: "Two Sum",
		MetaData: `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`}
	addNative(q, "golang", "func twoSum(nums []int, target int) []int { return nil }")
	return q
}

func addNative(q *Question, language, source string) {
	q.CodeSnippets = append(q.CodeSnippets, struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	}{language, source})
}

func saveTemplateProblem(t *testing.T, q *Question) *problem.Problem {
	t.Helper()
	c, err := Convert(q, SiteCom)
	if err != nil {
		t.Fatal(err)
	}
	c.Problem.Judge.Compare = problem.CompareUnordered
	c.Localize([]string{"en"}, "en")
	dir := filepath.Join(t.TempDir(), q.TitleSlug)
	if err := c.Save(dir); err != nil {
		t.Fatal(err)
	}
	p, err := problem.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTemplatePlan(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"add", "edited", "signature", "identity", "selected", "missing", "bad-c-unselected"} {
		t.Run(scenario, func(t *testing.T) {
			q := templateQuestion()
			p := saveTemplateProblem(t, q)
			addNative(q, "cpp", "class Solution {};")
			languages := []string{"go", "cpp", "java"}
			switch scenario {
			case "edited":
				if err := os.WriteFile(filepath.Join(p.Dir, "templates/solution.go"), []byte("// edited\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "signature":
				p.Function.Params[0].Type = "double[]"
			case "identity":
				p.ID = "42"
			case "selected":
				languages = []string{"cpp"}
			case "missing":
				languages = []string{"java"}
			case "bad-c-unselected":
				addNative(q, "c", "unparseable C")
			}
			u, err := PlanTemplates(p, q, SiteCom, languages)
			if err != nil {
				t.Fatal(err)
			}
			wantConflict := scenario == "edited" || scenario == "signature" || scenario == "identity"
			if (len(u.Conflicts) > 0) != wantConflict {
				t.Fatalf("conflicts: %v", u.Conflicts)
			}
			if scenario == "missing" {
				if len(u.Files) != 0 || len(u.Missing) != 1 {
					t.Fatal(u)
				}
			} else if len(u.Added) != 1 || u.Added[0] != "templates/solution.cpp" {
				t.Fatal(u)
			}
			if _, err := os.Stat(filepath.Join(p.Dir, "templates/solution.cpp")); !os.IsNotExist(err) {
				t.Fatal("plan wrote a file")
			}
			if _, ok := u.Files[problem.MetaFile]; ok {
				t.Fatal("non-C migration changed metadata")
			}
		})
	}
}

func TestTemplateCBindings(t *testing.T) {
	t.Parallel()
	for _, slug := range []string{"flatten-nested-list-iterator", "serialize-and-deserialize-binary-tree", "serialize-and-deserialize-bst"} {
		t.Run(slug, func(t *testing.T) {
			q := loadQuestion(t, slug)
			p := saveTemplateProblem(t, q)
			source := nativeCTemplates(t)[slug]
			if slug == "serialize-and-deserialize-bst" {
				source = nativeCTemplates(t)["serialize-and-deserialize-binary-tree"]
			}
			addNative(q, "c", source)
			fresh, err := Convert(q, SiteCom)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := os.ReadFile(filepath.Join(p.Dir, problem.MetaFile))
			if err != nil {
				t.Fatal(err)
			}
			for _, existing := range []string{"", "c: {libraries: [glib]} # keep my library\n", "c:\n  methods: {}\n"} {
				original := append([]byte("# curated 中文\n"), meta...)
				original = append(original, []byte(existing)...)
				path := filepath.Join(p.Dir, problem.MetaFile)
				if err := os.WriteFile(path, original, 0644); err != nil {
					t.Fatal(err)
				}
				p, err := problem.Load(p.Dir)
				if err != nil {
					t.Fatal(err)
				}
				u, err := PlanTemplates(p, q, SiteCN, []string{"c"})
				if err != nil || len(u.Conflicts) != 0 {
					t.Fatalf("%s: %v, %+v", existing, err, u)
				}
				updated := u.Files[problem.MetaFile]
				if !bytes.Contains(updated, meta) || !bytes.Contains(updated, []byte("# curated 中文")) {
					t.Fatalf("unrelated metadata changed:\n%s", updated)
				}
				if strings.Contains(existing, "keep my library") && !bytes.Contains(updated, []byte("keep my library")) {
					t.Fatalf("comment lost:\n%s", updated)
				}
				for rel, data := range u.Files {
					if err := os.WriteFile(filepath.Join(p.Dir, rel), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
				p, err = problem.Load(p.Dir)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(existing, "libraries") {
					p.C.Libraries = nil
				}
				if !reflect.DeepEqual(p.C, fresh.Problem.C) {
					t.Fatalf("got %+v, fresh %+v", p.C, fresh.Problem.C)
				}
				u, err = PlanTemplates(p, q, SiteCN, []string{"c"})
				if err != nil || len(u.Files) != 0 || len(u.Conflicts) != 0 {
					t.Fatalf("not idempotent: %v %+v", err, u)
				}
			}
		})
	}
}

func TestTemplateCConflict(t *testing.T) {
	t.Parallel()
	q := loadQuestion(t, "flatten-nested-list-iterator")
	p := saveTemplateProblem(t, q)
	addNative(q, "c", nativeCTemplates(t)[q.TitleSlug])
	meta, err := os.ReadFile(filepath.Join(p.Dir, problem.MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	meta = append(meta, []byte("c:\n  constructor: myCreate\n")...)
	if err := os.WriteFile(filepath.Join(p.Dir, problem.MetaFile), meta, 0644); err != nil {
		t.Fatal(err)
	}
	p, err = problem.Load(p.Dir)
	if err != nil {
		t.Fatal(err)
	}
	u, err := PlanTemplates(p, q, SiteCom, []string{"c"})
	if err != nil || len(u.Conflicts) != 1 || !strings.Contains(u.Conflicts[0], "c.constructor") {
		t.Fatalf("%v %+v", err, u)
	}
}

func TestTemplateCComments(t *testing.T) {
	t.Parallel()
	p := &problem.Problem{Kind: problem.KindFunction, Function: &problem.Function{Name: "f"}}
	for _, data := range []string{
		"---\n# title comment\ntitle: '中文'\n# C comment\nc: {libraries: [glib]} # library\n\n# trailing field comment\nslug: 'test' # slug\n...\n",
		"title: '中文'\nc:\n  libraries: [glib] # library\n# final comment\n",
		"title: '中文'\n...\n",
		"title: '中文'",
	} {
		out, _, err := mergeTemplateC([]byte(data), p, &problem.CBinding{Function: "nativeF"})
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(data, "\n") {
			if strings.Contains(line, "c:") || line == "" {
				continue
			}
			if !strings.Contains(string(out), line) {
				t.Fatalf("lost %q in:\n%s", line, out)
			}
		}
		if strings.Count(string(out), "# trailing field comment") != strings.Count(data, "# trailing field comment") {
			t.Fatalf("duplicated comment:\n%s", out)
		}
	}
}
