package ui

import (
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/termimg"
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
	md := NewMarkdown(ThemeLight, nil)
	src := "# Title\n\n**进阶：**你可以吗？\n"
	first := md.Render(src, "", 80)
	if strings.Contains(first, "**") {
		t.Errorf("bold markers not rendered:\n%s", first)
	}
	start := time.Now()
	for range 100 {
		if md.Render(src, "", 80) != first {
			t.Fatal("cached output differs")
		}
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("cached renders took %v", d)
	}
}

func TestMarkdownImages(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "assets", "ex1.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 320, 160))); err != nil {
		t.Fatal(err)
	}
	f.Close()

	src := "# T\n\n**示例 1：**\n\n![](assets/ex1.png)\n\n```\n输入：x\n```\n\n![tree](assets/missing.png) and ![](https://x.test/a.png)\n"
	md := NewMarkdown(ThemeDark, termimg.New(termimg.Blocks, io.Discard))
	width := 30
	out := md.Render(src, dir, width)
	if strings.Contains(out, "HCIMAGE") {
		t.Fatalf("image token left in output:\n%s", out)
	}
	blocks := 0
	for line := range strings.Lines(out) {
		if w := ansi.StringWidth(strings.TrimRight(line, "\n")); w > width {
			t.Errorf("line wider than %d: %d", width, w)
		}
		if strings.ContainsAny(line, "▀▄▌▐▖▗▘▝█") || strings.Contains(line, "48;2;") {
			blocks++
		}
	}
	// 320px at 8px per cell is 40 columns, fitted to 28 (30 minus glamour's
	// margin): 7 rows.
	if blocks != 7 {
		t.Errorf("%d image rows, want 7:\n%s", blocks, out)
	}
	plain := ansi.Strip(out)
	for _, want := range []string{"[image: tree]", "[image: a.png]", "输入：x"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
}
