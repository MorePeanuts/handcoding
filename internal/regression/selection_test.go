package regression

import "testing"

func TestSelection(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "go", "go,python"} {
		got, err := selection(value, []string{"go", "python"})
		if err != nil || !got["go"] || got["python"] != (value != "go") {
			t.Errorf("selection(%q) = %v, %v", value, got, err)
		}
	}
	for _, value := range []string{"g0", "go,", "go,unknown", " "} {
		if _, err := selection(value, []string{"go", "python"}); err == nil {
			t.Errorf("accepted invalid selection %q", value)
		}
	}
}

func TestMatrixSelection(t *testing.T) {
	t.Parallel()
	got, err := matrixSelection(`{"go":[],"c":["two-sum"]}`, []string{"go", "c", "java"}, []string{"two-sum", "lru-cache"})
	if err != nil || !got["go"]["lru-cache"] || !got["c"]["two-sum"] || got["c"]["lru-cache"] || got["java"] != nil {
		t.Fatalf("selection = %v, %v", got, err)
	}
	for _, value := range []string{`{}`, `null`, `{`, `{"rust":[]}`, `{"go":["missing"]}`, `{"go":[""]}`, `{"go":["two-sum,lru-cache"]}`} {
		if _, err := matrixSelection(value, []string{"go"}, []string{"two-sum", "lru-cache"}); err == nil {
			t.Errorf("accepted invalid selection %s", value)
		}
	}
}
