package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/regression"
	"github.com/MorePeanuts/handcoding/internal/workspace"
)

func TestShowMarkdown(t *testing.T) {
	ws, err := workspace.Create(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.Install(ws); err != nil {
		t.Fatal(err)
	}
	p, err := ws.Problem("two-sum")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		args []string
		file string
	}{
		{[]string{"show", "1"}, "description.md"},
		{[]string{"show", "two-sum", "--raw"}, "description.md"},
		{[]string{"show", "regression/two-sum", "--locale", "zh-CN"}, "description.zh-CN.md"},
		{[]string{"show", "two-sum", "--raw", "--locale", "zh-CN"}, "description.zh-CN.md"},
	} {
		want, err := os.ReadFile(filepath.Join(p.Dir, tt.file))
		if err != nil {
			t.Fatal(err)
		}
		out, err := capture(t, append([]string{"-w", ws.Root}, tt.args...)...)
		if err != nil || out != string(want)+"\n" {
			t.Errorf("%v: %v\noutput:\n%s", tt.args, err, out)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(ws.Root, "solutions")); err != nil || len(entries) != 0 {
		t.Fatalf("show created solutions: %v, %v", entries, err)
	}
	if _, err := capture(t, "-w", ws.Root, "show", "does-not-exist"); err == nil {
		t.Fatal("show accepted an unknown problem")
	}
	if _, err := capture(t, "-w", ws.Root, "show", "1", "--locale", "fr"); err == nil || !strings.Contains(err.Error(), "no locale") {
		t.Fatalf("show accepted an unknown locale: %v", err)
	}
}
