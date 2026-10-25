package termimg

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// writePNG writes a w×h image and returns its path.
func writePNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	path := filepath.Join(t.TempDir(), "img.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func checkGrid(t *testing.T, img Image) {
	t.Helper()
	lines := strings.Split(img.Text, "\n")
	if len(lines) != img.Rows {
		t.Fatalf("%d lines, want %d rows", len(lines), img.Rows)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != img.Cols {
			t.Fatalf("line %d is %d cells wide, want %d", i, w, img.Cols)
		}
	}
}

func TestRenderSize(t *testing.T) {
	path := writePNG(t, 160, 80)
	r := &Renderer{proto: Blocks, cellRatio: 0.5, sent: map[uint32]bool{}}
	// Natural size: 160px / 8px per cell = 20 columns, half as many rows
	// for the 2:1 image, halved again for the cell aspect.
	img, err := r.Render(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if img.Cols != 20 || img.Rows != 5 {
		t.Errorf("natural size %dx%d, want 20x5", img.Cols, img.Rows)
	}
	checkGrid(t, img)
	// Narrow terminals shrink the image.
	img, err = r.Render(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if img.Cols != 10 || img.Rows != 3 {
		t.Errorf("fitted size %dx%d, want 10x3", img.Cols, img.Rows)
	}
	checkGrid(t, img)
}

func TestKittyUploadsOnce(t *testing.T) {
	t.Setenv("TMUX", "")
	path := writePNG(t, 64, 64)
	var out bytes.Buffer
	r := &Renderer{proto: Kitty, out: &out, cellRatio: 0.5, sent: map[uint32]bool{}}
	img, err := r.Render(path, 80)
	if err != nil {
		t.Fatal(err)
	}
	checkGrid(t, img)
	if !strings.Contains(img.Text, "\U0010EEEE") {
		t.Error("no placeholder cells")
	}
	if out.Len() != 0 {
		t.Error("Render wrote to the terminal")
	}
	if err := r.Show(img, img); err != nil {
		t.Fatal(err)
	}
	upload := out.String()
	if strings.Count(upload, "U=1") != 1 || !strings.Contains(upload, "q=2") || !strings.HasPrefix(upload, "\x1b_G") {
		t.Errorf("unexpected upload: %.120q", upload)
	}
	if err := r.Show(img); err != nil {
		t.Fatal(err)
	}
	if out.Len() != len(upload) {
		t.Error("image uploaded twice")
	}
	r.Forget()
	if err := r.Show(img); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 2*len(upload) {
		t.Error("image not uploaded again after Forget")
	}
}

func TestDetect(t *testing.T) {
	for _, k := range []string{"TMUX", "KITTY_WINDOW_ID", "TERM_PROGRAM", "COLORTERM"} {
		t.Setenv(k, "")
	}
	t.Setenv("TERM", "xterm-256color")
	if p := Detect("auto", true); p != None {
		t.Errorf("plain terminal: %s", p)
	}
	t.Setenv("COLORTERM", "truecolor")
	if p := Detect("", true); p != Blocks {
		t.Errorf("truecolor terminal: %s", p)
	}
	t.Setenv("TERM_PROGRAM", "ghostty")
	if p := Detect("auto", true); p != Kitty {
		t.Errorf("ghostty: %s", p)
	}
	t.Setenv("TERM_PROGRAM", "herdr")
	if p := Detect("auto", true); p != Kitty {
		t.Errorf("herdr: %s", p)
	}
	t.Setenv("TERM_PROGRAM", "WezTerm")
	if p := Detect("auto", true); p != Blocks {
		t.Errorf("wezterm: %s", p)
	}
	t.Setenv("TERM_PROGRAM", "ghostty")
	if p := Detect("auto", false); p != None {
		t.Errorf("not a terminal: %s", p)
	}
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	if p := Detect("auto", true); p != Blocks {
		t.Errorf("ghostty in tmux: %s", p)
	}
	if p := Detect("kitty", true); p != Kitty {
		t.Errorf("configured: %s", p)
	}
}
