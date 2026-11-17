package cli

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

func snapshotTemplates(t *testing.T, dir string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		result[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFetchTemplatesMigration(t *testing.T) {
	for _, scenario := range []string{"migrate", "authoring", "dry-run", "conflict", "fetch-error", "batch", "installed"} {
		t.Run(scenario, func(t *testing.T) {
			snippets := []map[string]string{
				{"langSlug": "golang", "code": "func twoSum(nums []int, target int) []int {}"},
				{"langSlug": "python3", "code": "class Solution:\n    def twoSum(self, nums, target): pass\n"},
			}
			fakeLeetCode(t, func(q map[string]any) { q["codeSnippets"] = snippets })
			dir := localesWorkspace(t, "[en, zh-CN]")
			if _, err := capture(t, "-w", dir, "fetch", "--site", "cn", "two-sum"); err != nil {
				t.Fatal(err)
			}
			pdir := filepath.Join(dir, "packs/local/problems/0001-two-sum")
			packPath := filepath.Join(dir, "packs/local/pack.yaml")
			if scenario == "authoring" {
				dir = t.TempDir()
				packPath = filepath.Join(dir, "pack.yaml")
				if err := writeFile(packPath, "protocol: 1\nname: curated\nlanguages: [go, python]\nlocales: [en, zh-CN]\n"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, "problems"), 0755); err != nil {
					t.Fatal(err)
				}
				moved := filepath.Join(dir, "problems/0001-two-sum")
				if err := os.Rename(pdir, moved); err != nil {
					t.Fatal(err)
				}
				pdir = moved
			}
			for rel, content := range map[string]string{
				"reference/solution.go": "// curated reference\n", "reference/solution.py": "# curated reference\n",
				"checker.py": "# curated checker\n", "generator.py": "# curated generator\n",
				"cases.jsonl": "{\"id\":\"curated\"}\n",
			} {
				if err := writeFile(filepath.Join(pdir, rel), content); err != nil {
					t.Fatal(err)
				}
			}
			metaPath := filepath.Join(pdir, problem.MetaFile)
			meta, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			meta = append([]byte("# curated metadata\n"), meta...)
			meta = append(meta, []byte("judge: {compare: unordered} # preserve this layout\n")...)
			if err := os.WriteFile(metaPath, meta, 0644); err != nil {
				t.Fatal(err)
			}
			snippets = append(snippets,
				map[string]string{"langSlug": "c", "code": "int* nativeTwoSum(int* nums, int numsSize, int target, int* returnSize) {}"},
				map[string]string{"langSlug": "cpp", "code": "class Solution {};"},
				map[string]string{"langSlug": "javascript", "code": "var twoSum = function(nums, target) {};"},
				map[string]string{"langSlug": "typescript", "code": "function twoSum(nums: number[], target: number): number[] {return [];}"})
			args := []string{"-w", dir, "fetch", "--templates-only", "--site", "cn"}
			switch scenario {
			case "dry-run":
				args = append(args, "--dry-run")
			case "conflict":
				if err := writeFile(filepath.Join(pdir, "templates/solution.go"), "// edited template\n"); err != nil {
					t.Fatal(err)
				}
			case "fetch-error":
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("invalid JSON")) }))
				defer s.Close()
				leetcode.SiteCN.Base = s.URL
			case "batch":
				args = append(args, "999999999")
			case "installed":
				installed := filepath.Join(dir, "packs/installed")
				if err := os.MkdirAll(filepath.Join(installed, "problems"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := writeFile(filepath.Join(installed, "pack.yaml"), "protocol: 1\nname: installed\nlocales: [en, zh-CN]\n"); err != nil {
					t.Fatal(err)
				}
				moved := filepath.Join(installed, "problems/0001-two-sum")
				if err := os.Rename(pdir, moved); err != nil {
					t.Fatal(err)
				}
				pdir = moved
			}
			before := snapshotTemplates(t, pdir)
			packBefore, err := os.ReadFile(packPath)
			if err != nil {
				t.Fatal(err)
			}
			args = append(args, "two-sum")
			out, err := capture(t, args...)
			wantErr := scenario == "conflict" || scenario == "fetch-error" || scenario == "batch" || scenario == "installed"
			if (err != nil) != wantErr {
				t.Fatalf("%v\n%s", err, out)
			}
			after := snapshotTemplates(t, pdir)
			if scenario != "migrate" && scenario != "authoring" && scenario != "batch" {
				if !reflect.DeepEqual(before, after) {
					t.Fatal("failed/preview migration modified problem")
				}
			} else {
				for rel, data := range before {
					if rel == problem.MetaFile {
						if !strings.Contains(after[rel], data) {
							t.Fatalf("changed curated YAML:\n%s", after[rel])
						}
					} else if after[rel] != data {
						t.Fatalf("changed %s", rel)
					}
				}
				if len(after) != len(before)+4 {
					t.Fatalf("expected four new native templates: %v", after)
				}
				p, err := problem.Load(pdir)
				if err != nil || p.C.Function != "nativeTwoSum" {
					t.Fatalf("C binding: %v %+v", err, p)
				}
				if !strings.Contains(out, "unavailable java") {
					t.Fatalf("missing snippet not reported: %s", out)
				}
				if scenario == "migrate" {
					if _, err := capture(t, args...); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(after, snapshotTemplates(t, pdir)) {
						t.Fatal("repeat migration changed content")
					}
				}
			}
			packAfter, err := os.ReadFile(packPath)
			if err != nil || string(packAfter) != string(packBefore) {
				t.Fatal("pack config changed")
			}
			if scenario == "conflict" && !strings.Contains(out, "conflict templates/solution.go") {
				t.Fatalf("missing conflict: %s", out)
			}
		})
	}
}

func TestFetchTemplatesFlags(t *testing.T) {
	for _, flags := range [][]string{
		{"--dry-run"}, {"--lang", "c"}, {"--templates-only", "--force"}, {"--templates-only", "--lang", "rust"},
	} {
		if _, err := capture(t, append([]string{"fetch", "two-sum"}, flags...)...); err == nil {
			t.Fatalf("accepted %v", flags)
		}
	}
	fakeLeetCode(t, func(q map[string]any) {
		q["codeSnippets"] = []map[string]string{{"langSlug": "typescript", "code": "function twoSum(nums: number[], target: number): number[] {return [];}"}}
	})
	dir := localesWorkspace(t, "[en]")
	if _, err := capture(t, "-w", dir, "fetch", "two-sum"); err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(dir, "packs/local/problems/0001-two-sum")
	if err := os.Remove(filepath.Join(pdir, "templates/solution.ts")); err != nil {
		t.Fatal(err)
	}
	out, err := capture(t, "-w", dir, "fetch", "--templates-only", "-l", "js,ts", "-l", "typescript", "two-sum")
	if err != nil || !strings.Contains(out, "unavailable javascript") || strings.Contains(out, "unavailable go") {
		t.Fatalf("%v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(pdir, "templates/solution.ts")); err != nil {
		t.Fatal(err)
	}
}
