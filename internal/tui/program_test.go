package tui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"

	"github.com/MorePeanuts/handcoding/internal/termimg"
)

// The tests in this file run a real bubbletea Program, to check what hc
// relies on from it: the keys the screens match, and statement images
// reaching the terminal intact.

// keyRecorder records the keys a program receives, and quits after want.
type keyRecorder struct {
	keys []string
	want int
}

func (r *keyRecorder) Init() tea.Cmd { return nil }

func (r *keyRecorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		r.keys = append(r.keys, k.String())
		if len(r.keys) == r.want {
			return r, tea.Quit
		}
	}
	return r, nil
}

func (r *keyRecorder) View() tea.View { return tea.NewView("") }

// TestTerminalKeys checks the names that the keys the TUI matches get, as
// terminals send them: plain, with the kitty keyboard protocol (which
// bubbletea turns on) and with xterm's modifyOtherKeys.
func TestTerminalKeys(t *testing.T) {
	t.Parallel()
	keys := []struct{ in, want string }{
		{"j", "j"},
		{"G", "G"},
		{"/", "/"},
		{"Y", "Y"},
		{" ", "space"},
		{"\x15", "ctrl+u"},
		{"\x04", "ctrl+d"},
		{"\x03", "ctrl+c"},
		{"\t", "tab"},
		{"\x1b[Z", "shift+tab"},
		{"\r", "enter"},
		{"\x7f", "backspace"},
		{"\x1b[A", "up"},
		{"\x1b[B", "down"},
		{"\x1b[5~", "pgup"},
		{"\x1b[6~", "pgdown"},
		{"\x1b[H", "home"},
		{"\x1b[F", "end"},
		{"\x1b[3~", "delete"},
		// kitty keyboard protocol, disambiguated keys
		{"\x1b[117;5u", "ctrl+u"},
		{"\x1b[99;5u", "ctrl+c"},
		{"\x1b[9;2u", "shift+tab"},
		{"\x1b[27u", "esc"},
		{"\x1b[13u", "enter"},
		// modifyOtherKeys
		{"\x1b[27;5;100~", "ctrl+d"},
		// A lone escape last: it is told apart from a sequence by what
		// follows, here the end of input.
		{"\x1b", "esc"},
	}
	var in strings.Builder
	var want []string
	for _, k := range keys {
		in.WriteString(k.in)
		want = append(want, k.want)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := &keyRecorder{want: len(want)}
	p := tea.NewProgram(r, tea.WithContext(ctx), tea.WithInput(strings.NewReader(in.String())),
		tea.WithOutput(io.Discard), tea.WithWindowSize(80, 24), tea.WithoutSignals())
	if _, err := p.Run(); err != nil {
		t.Fatalf("%v; got keys %q", err, r.keys)
	}
	if !slices.Equal(r.keys, want) {
		t.Errorf("keys\n got %q\nwant %q", r.keys, want)
	}
}

// staticView shows content in the alternate screen, as the TUI does, and
// quits at once: the program draws it on its way out.
type staticView struct{ content string }

func (v staticView) Init() tea.Cmd                       { return tea.Quit }
func (v staticView) Update(tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v staticView) View() tea.View {
	view := tea.NewView(v.content)
	view.AltScreen = true
	return view
}

// TestProgramKeepsImageCells checks that a kitty image drawn in the TUI
// reaches the terminal as it was rendered: each placeholder cell with its
// row and column diacritics, in the 24-bit color that names the image, even
// where the environment reports 256 colors only.
func TestProgramKeepsImageCells(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "img.png")
	img := image.NewRGBA(image.Rect(0, 0, 48, 32))
	for y := range 32 {
		for x := range 48 {
			img.Set(x, y, color.RGBA{uint8(5 * x), uint8(7 * y), 200, 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	rendered, err := termimg.New(termimg.Kitty, io.Discard).Render(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Rows < 2 || rendered.Cols < 2 {
		t.Fatalf("image of %dx%d cells", rendered.Cols, rendered.Rows)
	}
	_, fg := parseCells(rendered.Text)
	if len(fg) == 0 || !strings.HasPrefix(fg[0], "#") {
		t.Fatalf("image color %q", fg)
	}
	var want []string
	for y := range rendered.Rows {
		for x := range rendered.Cols {
			want = append(want, string([]rune{kitty.Placeholder, kitty.Diacritic(y), kitty.Diacritic(x)})+" "+fg[0])
		}
	}

	// The image indented in a statement, as ui.Markdown draws it.
	var content strings.Builder
	content.WriteString("Statement text\n\n")
	for row := range strings.SplitSeq(rendered.Text, "\n") {
		content.WriteString("  " + row + "\n")
	}
	content.WriteString("\nmore text")
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := append(programOptions(ctx), tea.WithInput(nil), tea.WithOutput(&out), tea.WithWindowSize(80, 24),
		tea.WithoutSignals(), tea.WithEnvironment([]string{"TERM=xterm-256color"}))
	if _, err := tea.NewProgram(staticView{content.String()}, opts...).Run(); err != nil {
		t.Fatal(err)
	}
	cells, colors := parseCells(out.String())
	var got []string
	for i, c := range cells {
		if strings.HasPrefix(c, string(kitty.Placeholder)) {
			got = append(got, c+" "+colors[i])
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("placeholder cells written\n got %q\nwant %q\noutput %q", got, want, out.String())
	}
}

// parseCells returns the grapheme clusters printed in s, and the foreground
// color each was printed in ("" for the default, "#rrggbb" for a 24-bit one,
// the number otherwise). Cursor movements are ignored.
func parseCells(s string) (cells, fgs []string) {
	var (
		state byte
		fg    string
	)
	for len(s) > 0 {
		seq, width, n, newState := ansi.DecodeSequence(s, state, nil)
		state = newState
		switch {
		case width > 0:
			cells = append(cells, seq)
			fgs = append(fgs, fg)
		case strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m"):
			fg = sgrForeground(seq[2:len(seq)-1], fg)
		}
		s = s[n:]
	}
	return cells, fgs
}

// sgrForeground applies the parameters of an SGR sequence to a foreground
// color.
func sgrForeground(params, fg string) string {
	if strings.ContainsAny(params, "<=>?") {
		return fg
	}
	var ps []string
	for p := range strings.SplitSeq(params, ";") {
		// Sub-parameters (38:2::r:g:b) as parameters.
		sub := strings.Split(p, ":")
		if len(sub) == 6 && sub[1] == "2" {
			sub = append(sub[:2], sub[3:]...) // drop the color space
		}
		ps = append(ps, sub...)
	}
	for i := 0; i < len(ps); i++ {
		switch n, _ := strconv.Atoi(ps[i]); {
		case n == 0 || n == 39:
			fg = ""
		case n >= 30 && n <= 37 || n >= 90 && n <= 97:
			fg = ps[i]
		case n == 38 && i+1 < len(ps) && ps[i+1] == "5" && i+2 < len(ps):
			fg, i = ps[i+2], i+2
		case n == 38 && i+1 < len(ps) && ps[i+1] == "2" && i+4 < len(ps):
			var rgb [3]int
			for j := range rgb {
				rgb[j], _ = strconv.Atoi(ps[i+2+j])
			}
			fg, i = fmt.Sprintf("#%02x%02x%02x", rgb[0], rgb[1], rgb[2]), i+4
		case n == 48 && i+1 < len(ps) && ps[i+1] == "5":
			i += 2
		case n == 48 && i+1 < len(ps) && ps[i+1] == "2":
			i += 4
		}
	}
	return fg
}
