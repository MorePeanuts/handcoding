// Package termimg draws images in the terminal as plain text lines, so they
// can live inside Markdown output and scroll in a TUI viewport like any
// other text.
//
// Two protocols are supported:
//
//   - kitty: the image is uploaded once with the kitty graphics protocol as
//     a virtual placement, then drawn with Unicode placeholder cells
//     (kitty, Ghostty). Full resolution.
//   - blocks: the image is approximated with colored block characters.
//     Works in any truecolor terminal.
package termimg

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif" // decoders for statement images
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	"github.com/charmbracelet/x/mosaic"
	_ "golang.org/x/image/webp"
)

// Protocol selects how images are drawn. The values are accepted by
// hc.yaml's `images`.
type Protocol string

const (
	Auto   Protocol = "auto"
	Kitty  Protocol = "kitty"
	Blocks Protocol = "blocks"
	None   Protocol = "none"
)

// Detect resolves a configured protocol. "auto" (or empty) picks kitty in
// terminals known to support Unicode placeholders, blocks in other truecolor
// terminals, and none otherwise or when output is not a terminal.
func Detect(configured string, tty bool) Protocol {
	switch p := Protocol(configured); p {
	case Kitty, Blocks, None:
		return p
	}
	if !tty {
		return None
	}
	term, program := os.Getenv("TERM"), os.Getenv("TERM_PROGRAM")
	// Inside tmux the outer terminal is unknown and passthrough may be off;
	// kitty must be chosen explicitly there.
	if os.Getenv("TMUX") == "" && (os.Getenv("KITTY_WINDOW_ID") != "" || term == "xterm-kitty" ||
		program == "ghostty" || term == "xterm-ghostty") {
		return Kitty
	}
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return Blocks
	}
	return None
}

// Image is an image rendered for the terminal.
type Image struct {
	// Text draws the image: one line per terminal row, each exactly Cols
	// cells wide, without a trailing newline.
	Text       string
	Cols, Rows int

	// upload transmits the image (kitty only); sent once per terminal.
	id     uint32
	upload string
}

// Renderer renders images for one terminal.
type Renderer struct {
	proto Protocol
	out   io.Writer
	// cellRatio is a terminal cell's width divided by its height.
	cellRatio float64

	mu   sync.Mutex
	sent map[uint32]bool
}

// New creates a renderer. Kitty uploads are written to out, which must be
// the terminal.
func New(proto Protocol, out io.Writer) *Renderer {
	return &Renderer{proto: proto, out: out, cellRatio: cellRatio(out), sent: map[uint32]bool{}}
}

// Protocol returns the protocol in use.
func (r *Renderer) Protocol() Protocol { return r.proto }

// ErrDisabled is returned by Render when images are not drawn.
var ErrDisabled = errors.New("terminal images disabled")

// pxPerCell maps image pixels to terminal columns when the image is small
// enough to be shown at its natural size: statement images are sized for
// web pages, where a column of monospace text is about 8px wide.
const pxPerCell = 8

const (
	// defaultCellRatio is used when the terminal does not report its cell
	// size: cells are about twice as tall as they are wide.
	defaultCellRatio = 0.5
	// maxRows keeps rows within kitty's table of row diacritics.
	maxRows = 256
)

// Render draws the image at path in at most maxCols columns.
func (r *Renderer) Render(path string, maxCols int) (Image, error) {
	if r == nil || r.proto == None {
		return Image{}, ErrDisabled
	}
	f, err := os.Open(path)
	if err != nil {
		return Image{}, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return Image{}, fmt.Errorf("%s: %w", path, err)
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return Image{}, fmt.Errorf("%s: empty image", path)
	}
	cols := min(max(maxCols, 1), int(math.Ceil(float64(b.Dx())/pxPerCell)))
	rows := int(math.Round(float64(cols) * float64(b.Dy()) / float64(b.Dx()) * r.cellRatio))
	if rows > maxRows {
		cols = max(cols*maxRows/rows, 1)
		rows = maxRows
	}
	rows = max(rows, 1)

	if r.proto == Blocks {
		// mosaic draws 2×2 source pixels per cell.
		m := mosaic.New().Width(2 * cols).Height(2 * rows).Symbol(mosaic.Quarter)
		text := m.Render(img)
		return Image{Text: strings.TrimSuffix(text, "\n"), Cols: cols, Rows: rows}, nil
	}
	return r.kitty(path, img, cols, rows)
}

func (r *Renderer) kitty(path string, img image.Image, cols, rows int) (Image, error) {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s\x00%d\x00%d", path, cols, rows)
	// The ID is carried by the 24-bit foreground color of the placeholders.
	id := h.Sum32() & 0xffffff
	if id == 0 {
		id = 1
	}
	var upload bytes.Buffer
	opts := &kitty.Options{
		Action:           kitty.TransmitAndPut,
		Transmission:     kitty.Direct,
		Format:           kitty.PNG,
		Quiet:            2, // no replies: they would arrive as keystrokes
		ID:               int(id),
		Columns:          cols,
		Rows:             rows,
		VirtualPlacement: true,
		Chunk:            true,
	}
	if os.Getenv("TMUX") != "" {
		opts.ChunkFormatter = ansi.TmuxPassthrough
	}
	if err := kitty.EncodeGraphics(&upload, img, opts); err != nil {
		return Image{}, err
	}

	color := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", id>>16&0xff, id>>8&0xff, id&0xff)
	var text strings.Builder
	for y := range rows {
		if y > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(color)
		for x := range cols {
			text.WriteRune(kitty.Placeholder)
			text.WriteRune(kitty.Diacritic(y))
			text.WriteRune(kitty.Diacritic(x))
		}
		text.WriteString("\x1b[39m")
	}
	return Image{Text: text.String(), Cols: cols, Rows: rows, id: id, upload: upload.String()}, nil
}

// Show uploads the images the terminal does not have yet. Call it before
// the images' text is drawn.
func (r *Renderer) Show(imgs ...Image) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var buf bytes.Buffer
	for _, img := range imgs {
		if img.upload == "" || r.sent[img.id] {
			continue
		}
		buf.WriteString(img.upload)
		r.sent[img.id] = true
	}
	if buf.Len() == 0 {
		return nil
	}
	// One write: other goroutines (the TUI renderer) write to the same
	// terminal, and writes to one file are serialized.
	_, err := r.out.Write(buf.Bytes())
	return err
}

// Forget marks every image as not uploaded, for when the terminal may have
// dropped them (e.g. after another program used the alternate screen).
func (r *Renderer) Forget() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.sent)
}
