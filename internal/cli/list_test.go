package cli

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestFormatTableAlignsWideText(t *testing.T) {
	out := formatTable([][]string{
		{"ID", "TITLE", "LEVEL"},
		{"1", "Two Sum", "easy"},
		{"26", "删除有序数组中的重复项", "easy"},
	})
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	col := func(line string) int { return runewidth.StringWidth(line[:strings.LastIndex(line, " ")+1]) }
	if a, b, c := col(lines[0]), col(lines[1]), col(lines[2]); a != b || b != c {
		t.Errorf("last column starts at display widths %d, %d, %d:\n%s", a, b, c, out)
	}
}
