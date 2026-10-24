package ui

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/termimg"
)

// Theme names accepted by hc.yaml's `theme`.
const (
	ThemeAuto  = "auto"
	ThemeDark  = "dark"
	ThemeLight = "light"
)

// DetectTheme resolves a configured theme. "auto" (or empty) asks the
// terminal for its background color, which must happen before a TUI takes
// over stdin: inside bubbletea the terminal's reply is swallowed and every
// query waits for its timeout.
func DetectTheme(configured string) string {
	switch configured {
	case ThemeDark, ThemeLight:
		return configured
	}
	if lipgloss.HasDarkBackground() {
		return ThemeDark
	}
	return ThemeLight
}

// Markdown renders Markdown for the terminal, caching renderers per width
// and output per (width, dir, source).
type Markdown struct {
	theme     string
	images    *termimg.Renderer
	mu        sync.Mutex
	renderers map[int]*glamour.TermRenderer
	cache     map[string]rendered
}

type rendered struct {
	out    string
	images []termimg.Image
}

// NewMarkdown creates a renderer for a resolved theme (dark or light).
// images draws embedded images; nil shows a text placeholder instead.
func NewMarkdown(theme string, images *termimg.Renderer) *Markdown {
	if theme != ThemeLight {
		theme = ThemeDark
	}
	return &Markdown{theme: theme, images: images, renderers: map[int]*glamour.TermRenderer{}, cache: map[string]rendered{}}
}

// Render renders src wrapped at width. Relative image paths are resolved
// against dir. On failure it returns src unchanged.
//
// The output must be drawn right away: images it contains are uploaded to
// the terminal (if the protocol needs it) before Render returns.
func (m *Markdown) Render(src, dir string, width int) string {
	width = max(width, 20)
	key := strconv.Itoa(width) + "\x00" + dir + "\x00" + src
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.cache[key]
	if !ok {
		var err error
		if r, err = m.render(src, dir, width); err != nil {
			return src
		}
		m.cache[key] = r
	}
	_ = m.images.Show(r.images...) // a failed upload leaves blank cells
	return r.out
}

// imageToken stands in for the i-th image while glamour renders the text.
func imageToken(i int) string { return fmt.Sprintf("HCIMAGE%dX", i) }

func (m *Markdown) render(src, dir string, width int) (rendered, error) {
	gr, ok := m.renderers[width]
	if !ok {
		var err error
		gr, err = glamour.NewTermRenderer(glamour.WithStandardStyle(m.theme), glamour.WithWordWrap(width))
		if err != nil {
			return rendered{}, err
		}
		m.renderers[width] = gr
	}
	// Images become paragraphs of their own, replaced after rendering.
	type ref struct{ alt, src string }
	var refs []ref
	src = problem.ReplaceImages(src, func(alt, src string) string {
		refs = append(refs, ref{alt, src})
		return "\n\n" + imageToken(len(refs)-1) + "\n\n"
	})
	out, err := gr.Render(FixCJKEmphasis(src))
	if err != nil || len(refs) == 0 {
		return rendered{out: out}, err
	}
	tokens := make(map[string]int, len(refs))
	for i := range refs {
		tokens[imageToken(i)] = i
	}
	var (
		b    strings.Builder
		imgs []termimg.Image
	)
	for line := range strings.Lines(out) {
		plain := ansi.Strip(line)
		i, ok := tokens[strings.TrimSpace(plain)]
		if !ok {
			b.WriteString(line)
			continue
		}
		indent := strings.Repeat(" ", len(plain)-len(strings.TrimLeft(plain, " ")))
		img, err := m.image(dir, refs[i].src, width-len(indent))
		if err != nil {
			label := refs[i].alt
			if label == "" {
				label = path.Base(refs[i].src)
			}
			b.WriteString(indent + Faint.Render("[image: "+label+"]") + "\n")
			continue
		}
		imgs = append(imgs, img)
		for row := range strings.SplitSeq(img.Text, "\n") {
			b.WriteString(indent + row + "\n")
		}
	}
	return rendered{out: b.String(), images: imgs}, nil
}

func (m *Markdown) image(dir, src string, maxCols int) (termimg.Image, error) {
	if m.images == nil {
		return termimg.Image{}, termimg.ErrDisabled
	}
	if strings.Contains(src, "://") {
		// Statements are rendered offline; hc fetch downloads images.
		return termimg.Image{}, fmt.Errorf("remote image %s", src)
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, filepath.FromSlash(src))
	}
	return m.images.Render(src, maxCols)
}

// cjkEmphasisRE matches **text** whose closing delimiter follows a
// punctuation mark, e.g. "**进阶：**你". CommonMark does not treat such a
// delimiter as closing when a letter follows, which is common in Chinese.
var cjkEmphasisRE = regexp.MustCompile(`\*\*([^*\n]+?)([：:，,。；;！!？?、）)])\*\*`)

// FixCJKEmphasis moves trailing punctuation out of bold spans so that they
// render: "**进阶：**你" becomes "**进阶**：你".
func FixCJKEmphasis(src string) string {
	return cjkEmphasisRE.ReplaceAllString(src, "**$1**$2")
}
