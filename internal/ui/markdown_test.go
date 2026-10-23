package ui

import (
	"strings"
	"testing"
	"time"
)

func TestFixCJKEmphasis(t *testing.T) {
	tests := map[string]string{
		"**进阶：**你可以":        "**进阶**：你可以",
		"**示例 1：**":         "**示例 1**：",
		"**Note:** text":    "**Note**: text",
		"**plain** text":    "**plain** text",
		"a **b** and **c**": "a **b** and **c**",
	}
	for in, want := range tests {
		if got := FixCJKEmphasis(in); got != want {
			t.Errorf("FixCJKEmphasis(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkdownCachesOutput(t *testing.T) {
	md := NewMarkdown(ThemeLight)
	src := "# Title\n\n**进阶：**你可以吗？\n"
	first := md.Render(src, 80)
	if strings.Contains(first, "**") {
		t.Errorf("bold markers not rendered:\n%s", first)
	}
	start := time.Now()
	for range 100 {
		if md.Render(src, 80) != first {
			t.Fatal("cached output differs")
		}
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("cached renders took %v", d)
	}
}
