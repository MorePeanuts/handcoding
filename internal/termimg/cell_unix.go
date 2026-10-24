//go:build unix

package termimg

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// cellRatio asks the terminal behind out for its cell size in pixels.
func cellRatio(out io.Writer) float64 {
	if f, ok := out.(*os.File); ok {
		ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
		if err == nil && ws.Xpixel > 0 && ws.Ypixel > 0 && ws.Col > 0 && ws.Row > 0 {
			return (float64(ws.Xpixel) / float64(ws.Col)) / (float64(ws.Ypixel) / float64(ws.Row))
		}
	}
	return defaultCellRatio
}
