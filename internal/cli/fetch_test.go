package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

// fakeLeetCode serves two-sum as the site does: leetcode.com in English,
// leetcode.cn with a Chinese translation too. Both statements show the same
// image, from a different URL on each site. It replaces both sites until the
// test ends.
func fakeLeetCode(t *testing.T) {
	t.Helper()
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	serve := func(cn bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/img/en.png", "/img/zh.png":
				_, _ = w.Write(img.Bytes())
				return
			case "/graphql/":
			default:
				http.NotFound(w, r)
				return
			}
			q := map[string]any{
				"questionId": "1", "questionFrontendId": "1", "title": "Two Sum", "titleSlug": "two-sum",
				"content":          `<p>Return the indices.</p><img src="/img/en.png"><pre><strong>Output:</strong> [0,1]</pre>`,
				"difficulty":       "Easy",
				"metaData":         `{"name":"twoSum","params":[{"name":"nums","type":"integer[]"},{"name":"target","type":"integer"}],"return":{"type":"integer[]"}}`,
				"exampleTestcases": "[2,7]\n9",
			}
			if cn {
				q["translatedTitle"] = "两数之和"
				q["translatedContent"] = `<p>返回下标。</p><img src="/img/zh.png"><pre><strong>输出：</strong>[0,1]</pre>`
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"question": q}})
		}))
	}
	com, cn := serve(false), serve(true)
	t.Cleanup(com.Close)
	t.Cleanup(cn.Close)
	oldCom, oldCN := leetcode.SiteCom, leetcode.SiteCN
	leetcode.SiteCom.Base, leetcode.SiteCN.Base = com.URL, cn.URL
	t.Cleanup(func() { leetcode.SiteCom, leetcode.SiteCN = oldCom, oldCN })
}

// localesWorkspace creates a workspace whose own problems are in locales.
func localesWorkspace(t *testing.T, locales string) string {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, workspace.ConfigFile), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString("locales: " + locales + "\n"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFetchLocales(t *testing.T) {
	fakeLeetCode(t)
	dir := localesWorkspace(t, "[en, zh-CN]")
	pdir := filepath.Join(dir, "packs", "local", "problems", "0001-two-sum")
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(pdir, name))
		if err != nil {
			return ""
		}
		return string(data)
	}

	// leetcode.com has English only.
	if _, err := capture(t, "-w", dir, "fetch", "--site", "com", "two-sum"); err != nil {
		t.Fatal(err)
	}
	if desc := read(problem.DescriptionFile); !strings.Contains(desc, "Return the indices.") || !strings.Contains(desc, "![](assets/en.png)") {
		t.Errorf("description.md:\n%s", desc)
	}
	if read("description.zh-CN.md") != "" {
		t.Error("leetcode.com has no Chinese statement")
	}
	// Nothing more to add from the same site.
	if _, err := capture(t, "-w", dir, "fetch", "--site", "com", "two-sum"); err == nil {
		t.Error("fetching again from leetcode.com should fail")
	}

	// leetcode.cn adds the Chinese statement and title, without --force,
	// and keeps the rest.
	if err := os.WriteFile(filepath.Join(pdir, problem.CasesFile), []byte(`{"input":{"nums":[1,2],"target":3},"expected":[0,1]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := capture(t, "-w", dir, "fetch", "--site", "cn", "two-sum")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "added zh-CN") {
		t.Errorf("output:\n%s", out)
	}
	// The image is the same file, from another URL.
	if zh := read("description.zh-CN.md"); !strings.Contains(zh, "返回下标。") || !strings.Contains(zh, "![](assets/en.png)") || !strings.Contains(zh, "# 1. 两数之和") {
		t.Errorf("description.zh-CN.md:\n%s", zh)
	}
	if entries, _ := os.ReadDir(filepath.Join(pdir, problem.AssetsDir)); len(entries) != 1 {
		t.Errorf("assets: %d files, want 1", len(entries))
	}
	if !strings.Contains(read(problem.CasesFile), `"nums":[1,2]`) {
		t.Error("adding a locale replaced cases.jsonl")
	}
	p, err := problem.Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Two Sum" || p.Titles["zh-CN"] != "两数之和" {
		t.Errorf("titles: %q, %v", p.Title, p.Titles)
	}
	if out, err := capture(t, "-w", dir, "validate", "two-sum"); err != nil {
		t.Errorf("validate: %v\n%s", err, out)
	}

	// hc show takes the locale from --locale, else the one remembered.
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"show", "two-sum"}, "Return the indices."},
		{[]string{"show", "two-sum", "--locale", "zh-CN"}, "返回下标。"},
	} {
		out, err := capture(t, append([]string{"-w", dir}, tt.args...)...)
		if err != nil || !strings.Contains(out, tt.want) {
			t.Errorf("%v: %v\n%s", tt.args, err, out)
		}
	}
	if _, err := capture(t, "-w", dir, "show", "two-sum", "--locale", "fr"); err == nil || !strings.Contains(err.Error(), "no locale \"fr\"") {
		t.Errorf("show --locale fr: %v", err)
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(ws)
	k, err := ws.Pack("local")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetLocale(context.Background(), k, "zh-CN"); err != nil {
		t.Fatal(err)
	}
	a.Close()
	if out, err := capture(t, "-w", dir, "show", "two-sum"); err != nil || !strings.Contains(out, "返回下标。") {
		t.Errorf("show with zh-CN remembered: %v\n%s", err, out)
	}
}

func TestFetchDefaultLocale(t *testing.T) {
	fakeLeetCode(t)
	// The default locale's statement must exist: leetcode.com has no
	// Chinese one.
	dir := localesWorkspace(t, "[zh-CN]")
	if _, err := capture(t, "-w", dir, "fetch", "--site", "com", "two-sum"); err == nil {
		t.Error("fetching a zh-CN pack from leetcode.com should fail")
	}
	if _, err := capture(t, "-w", dir, "fetch", "--site", "cn", "two-sum"); err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(dir, "packs", "local", "problems", "0001-two-sum")
	desc, _ := os.ReadFile(filepath.Join(pdir, problem.DescriptionFile))
	p, err := problem.Load(pdir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(desc), "返回下标。") || p.Title != "两数之和" || len(p.Titles) != 0 {
		t.Errorf("title %q, titles %v, description.md:\n%s", p.Title, p.Titles, desc)
	}

	// Without locales in hc.yaml the local pack is in English, from either
	// site.
	t.Setenv("PATH", t.TempDir())
	dir = filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := capture(t, "-w", dir, "fetch", "--site", "cn", "two-sum"); err != nil {
		t.Fatal(err)
	}
	pdir = filepath.Join(dir, "packs", "local", "problems", "0001-two-sum")
	desc, _ = os.ReadFile(filepath.Join(pdir, problem.DescriptionFile))
	if !strings.Contains(string(desc), "Return the indices.") {
		t.Errorf("description.md:\n%s", desc)
	}
	if _, err := os.Stat(filepath.Join(pdir, "description.zh-CN.md")); err == nil {
		t.Error("a pack in en only got a zh-CN statement")
	}
}

// hc import checks a problem against the locales of the pack it goes to,
// and copies its translations; hc validate reports what a locale lacks.
func TestImportLocales(t *testing.T) {
	src := filepath.Join(t.TempDir(), "two-sum")
	for name, content := range map[string]string{
		problem.MetaFile:        "slug: two-sum\ntitle: Two Sum\ntitles: {zh-CN: 两数之和}\nkind: function\nfunction:\n  name: twoSum\n  params:\n    - {name: nums, type: \"integer[]\"}\n  returns: integer\njudge:\n  compare: exact\n",
		problem.DescriptionFile: "# Two Sum\n",
		"description.zh-CN.md":  "# 两数之和\n",
		problem.CasesFile:       `{"input":{"nums":[1]},"expected":1}` + "\n",
	} {
		if err := writeFile(filepath.Join(src, name), content); err != nil {
			t.Fatal(err)
		}
	}
	// The local pack is in English only by default.
	t.Setenv("PATH", t.TempDir())
	plain := filepath.Join(t.TempDir(), "ws")
	if err := runHC(t, "init", plain); err != nil {
		t.Fatal(err)
	}
	if _, err := capture(t, "-w", plain, "import", src); err == nil {
		t.Error("importing a zh-CN translation into a pack in en only should fail")
	}

	dir := localesWorkspace(t, "[en, zh-CN]")
	if _, err := capture(t, "-w", dir, "import", src); err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(dir, "packs", "local", "problems", "two-sum")
	if data, err := os.ReadFile(filepath.Join(pdir, "description.zh-CN.md")); err != nil || string(data) != "# 两数之和\n" {
		t.Errorf("translation not imported: %q, %v", data, err)
	}
	if err := os.Remove(filepath.Join(pdir, "description.zh-CN.md")); err != nil {
		t.Fatal(err)
	}
	out, err := capture(t, "-w", dir, "validate", "two-sum")
	if err == nil || !strings.Contains(out, "no statement in zh-CN: description.zh-CN.md is missing") {
		t.Errorf("validate: %v\n%s", err, out)
	}
}
