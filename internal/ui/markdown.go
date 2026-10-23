package ui

import (
	"regexp"
	"strconv"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
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
// and output per (width, source).
type Markdown struct {
	theme     string
	mu        sync.Mutex
	renderers map[int]*glamour.TermRenderer
	cache     map[string]string
}

// NewMarkdown creates a renderer for a resolved theme (dark or light).
func NewMarkdown(theme string) *Markdown {
	if theme != ThemeLight {
		theme = ThemeDark
	}
	return &Markdown{theme: theme, renderers: map[int]*glamour.TermRenderer{}, cache: map[string]string{}}
}

// Render renders src wrapped at width. On failure it returns src unchanged.
func (m *Markdown) Render(src string, width int) string {
	width = max(width, 20)
	key := strconv.Itoa(width) + "\x00" + src
	m.mu.Lock()
	defer m.mu.Unlock()
	if out, ok := m.cache[key]; ok {
		return out
	}
	r, ok := m.renderers[width]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(glamour.WithStandardStyle(m.theme), glamour.WithWordWrap(width))
		if err != nil {
			return src
		}
		m.renderers[width] = r
	}
	out, err := r.Render(FixCJKEmphasis(src))
	if err != nil {
		return src
	}
	m.cache[key] = out
	return out
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
