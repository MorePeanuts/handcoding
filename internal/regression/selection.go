package regression

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// Select limits expensive corpus runs explicitly, never by tool availability.
// Empty variables mean the complete corpus. Unknown selections fail the test
// instead of silently passing with no work. An empty slug asks about a language.
func Select(t testing.TB) func(language, slug string) bool {
	t.Helper()
	var slugs []string
	for _, corpus := range Packs() {
		probs, loadErrors, err := pack.LoadProblems(filepath.Join(corpus.Dir, pack.ProblemsDir), corpus.Name)
		if err != nil {
			t.Fatal(err)
		}
		if len(probs) == 0 {
			t.Fatal("no problems in the regression set")
		}
		if len(loadErrors) != 0 {
			t.Fatal(loadErrors)
		}
		for _, p := range probs {
			slugs = append(slugs, p.Slug)
		}
	}
	if value := os.Getenv("HC_TEST_SELECTION"); value != "" {
		if os.Getenv("HC_TEST_LANGUAGES") != "" || os.Getenv("HC_TEST_PROBLEMS") != "" {
			t.Fatal("HC_TEST_SELECTION cannot be combined with HC_TEST_LANGUAGES/HC_TEST_PROBLEMS")
		}
		selected, err := matrixSelection(value, problem.Languages(problem.Protocol), slugs)
		if err != nil {
			t.Fatal(err)
		}
		return func(language, slug string) bool {
			problems, ok := selected[language]
			return ok && (slug == "" || problems[slug])
		}
	}
	languages, err := selection(os.Getenv("HC_TEST_LANGUAGES"), problem.Languages(problem.Protocol))
	if err != nil {
		t.Fatal(err)
	}
	problems, err := selection(os.Getenv("HC_TEST_PROBLEMS"), slugs)
	if err != nil {
		t.Fatal(err)
	}
	return func(language, slug string) bool {
		return languages[language] && (slug == "" || problems[slug])
	}
}

func selection(value string, allowed []string) (map[string]bool, error) {
	known := make(map[string]bool, len(allowed))
	for _, item := range allowed {
		known[item] = true
	}
	if value == "" {
		return known, nil
	}
	selected := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		if !known[item] {
			return nil, fmt.Errorf("unknown regression selection %q (allowed: %s)", item, strings.Join(allowed, ","))
		}
		selected[item] = true
	}
	return selected, nil
}

func matrixSelection(value string, languages, slugs []string) (map[string]map[string]bool, error) {
	var matrix map[string][]string
	if err := json.Unmarshal([]byte(value), &matrix); err != nil {
		return nil, err
	}
	if len(matrix) == 0 {
		return nil, fmt.Errorf("empty regression selection")
	}
	known, _ := selection("", languages)
	selected := make(map[string]map[string]bool)
	for language, problems := range matrix {
		if !known[language] {
			return nil, fmt.Errorf("unknown regression language %q", language)
		}
		for _, slug := range problems {
			if slug == "" || strings.Contains(slug, ",") {
				return nil, fmt.Errorf("invalid regression problem %q", slug)
			}
		}
		values, err := selection(strings.Join(problems, ","), slugs)
		if err != nil {
			return nil, err
		}
		selected[language] = values
	}
	return selected, nil
}
