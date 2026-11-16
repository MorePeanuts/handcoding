package leetcode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	clang "github.com/MorePeanuts/handcoding/internal/lang/c"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func nativeCTemplates(t *testing.T) map[string]string {
	t.Helper()
	b, e := os.ReadFile("testdata/c-templates.json")
	if e != nil {
		t.Fatal(e)
	}
	var templates map[string]string
	if e = json.Unmarshal(b, &templates); e != nil {
		t.Fatal(e)
	}
	return templates
}
func TestNativeCTemplateBindings(t *testing.T) {
	t.Parallel()
	templates := nativeCTemplates(t)
	dirs, e := os.ReadDir(regression.ProblemsDir())
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range dirs {
		p, e := problem.Load(filepath.Join(regression.ProblemsDir(), d.Name()))
		if e != nil {
			t.Fatal(e)
		}
		source, ok := templates[p.Slug]
		if !ok {
			continue
		}
		t.Run(p.Slug, func(t *testing.T) {
			if e := cBindings(p, source); e != nil {
				t.Fatal(e)
			}
			if e := p.Validate(); e != nil {
				t.Fatal(e)
			}
			switch p.Slug {
			case "flatten-nested-list-iterator":
				if p.C.Constructor != "nestedIterCreate" || p.C.Object != "struct" {
					t.Fatal(p.C)
				}
			case "serialize-and-deserialize-binary-tree":
				if p.C.Object != "none" {
					t.Fatal(p.C)
				}
			}
		})
	}
}
func TestNativeCTemplatesCompile(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, clang.DefaultCompiler())
	l := clang.Language{Environment: filepath.Join(t.TempDir(), "environment"), GLib: "disabled"}
	if e := l.Setup(context.Background(), os.Stderr); e != nil {
		t.Fatal(e)
	}
	slots := make(chan struct{}, 4)
	templates := nativeCTemplates(t)
	dirs, e := os.ReadDir(regression.ProblemsDir())
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range dirs {
		p, e := problem.Load(filepath.Join(regression.ProblemsDir(), d.Name()))
		if e != nil {
			t.Fatal(e)
		}
		source, ok := templates[p.Slug]
		if !ok {
			continue
		}
		t.Run(p.Slug, func(t *testing.T) {
			t.Parallel()
			slots <- struct{}{}
			defer func() { <-slots }()
			if e := cBindings(p, source); e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(t.TempDir(), "solution.c")
			if e := os.WriteFile(file, []byte(source), 0644); e != nil {
				t.Fatal(e)
			}
			if e := l.Prepare(filepath.Dir(file), p); e != nil {
				t.Fatal(e)
			}
			if e := l.Build(context.Background(), filepath.Dir(file), filepath.Dir(file)); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestConvertCBindings(t *testing.T) {
	t.Parallel()
	source := `typedef struct{int n;} Counter;
Counter* counterCreate(int start){ }
int counterNext(Counter*obj){ }
void counterFree(Counter*obj){ }
`
	q := &Question{QuestionFrontendID: "999", TitleSlug: "counter", Title: "Counter", MetaData: `{"classname":"Counter","constructor":{"params":[{"name":"start","type":"integer"}]},"methods":[{"name":"next","params":[],"return":{"type":"integer"}}]}`}
	q.CodeSnippets = append(q.CodeSnippets, struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	}{"c", source})
	converted, e := Convert(q, SiteCom)
	if e != nil {
		t.Fatal(e)
	}
	if converted.Templates["solution.c"] != source || converted.Problem.C == nil || converted.Problem.C.Constructor != "counterCreate" {
		t.Fatalf("%+v", converted)
	}
}
