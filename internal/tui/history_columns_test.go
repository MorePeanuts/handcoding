package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/MorePeanuts/handcoding/internal/store"
)

func TestWorkspaceHistoryColumns(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	addLocal(t, m, "0001-two-sum")
	refs := []string{"regression/two-sum", "local/two-sum", "regression/lru-cache", "regression/maximum-depth-of-binary-tree", "regression/中文题目", "missing/two-sum"}
	addSubs(t, m, "", refs...)
	// Exercise optional and nonnumeric IDs using the already-loaded metadata.
	for _, tab := range m.tabs {
		for _, p := range tab.probs {
			switch p.Ref() {
			case "regression/lru-cache":
				p.ID = ""
			case "regression/maximum-depth-of-binary-tree":
				p.ID = "LCR 001"
			}
		}
	}
	h.press("h")
	at := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	elapsed := int64(1)
	for i := range m.subs {
		m.subs[i].CreatedAt = at.Add(-time.Duration(i) * time.Hour)
		m.subs[i].ElapsedMs = &elapsed
		m.subs[i].Note = store.Note{Comment: "复习 notes"}
	}
	for _, width := range []int{160, 120, 80, 60, 40} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
		view := h.view()
		snapshot := strings.Split(view, "\n")
		snapshot[0] = " hc   /workspace"
		golden(t, fmt.Sprintf("history-all-%d", width), strings.Join(snapshot, "\n"))
		lines := strings.Split(ansi.Strip(view), "\n")
		if len(lines) != 20 {
			t.Fatalf("width %d: got %d lines", width, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > width {
				t.Fatalf("width %d: overflow: %q", width, line)
			}
		}
		header := lines[3]
		probStart := strings.Index(header, "PROBLEM")
		verdictStart := strings.Index(header, "VERDICT")
		if !strings.HasPrefix(header, "  ID ") || probStart < 0 || verdictStart <= probStart {
			t.Fatalf("width %d: missing primary columns: %s", width, header)
		}
		for i, want := range []string{"1", "1", "—", "LCR 001", "—", "—"} {
			line := lines[4+i]
			if got := strings.TrimSpace(ansi.Cut(line, 2, probStart)); got != want {
				t.Fatalf("width %d, %s: ID %q, want %q", width, refs[i], got, want)
			}
			if got := ansi.Cut(line, verdictStart, verdictStart+12); got != "Wrong Answer" {
				t.Fatalf("width %d: verdict obscured or columns misaligned: %q", width, got)
			}
		}
		if width >= 120 {
			order := []string{"ID", "PROBLEM", "VERDICT", "LANG", "PASSED", "TIME", "SUBMITTED", "SUB #", "COMMENT"}
			previous := -1
			for _, title := range order {
				if pos := strings.Index(header, title); pos <= previous {
					t.Fatalf("column %s misplaced: %s", title, header)
				} else {
					previous = pos
				}
			}
			for i, ref := range refs {
				pack, _, _ := strings.Cut(ref, "/")
				if !strings.Contains(lines[4+i], pack+"/") || width == 160 && !strings.Contains(lines[4+i], ref) {
					t.Fatalf("pack qualification lost: %s", lines[4+i])
				}
			}
		}
		if width <= 60 {
			for _, title := range []string{"TIME", "PASSED", "SUB #"} {
				if strings.Contains(header, title) {
					t.Fatalf("width %d: secondary column %s displaced primary content", width, title)
				}
			}
		}
	}
	// Narrow layouts hide the submission ID, but opening a row still reveals
	// that submission's full ID; rendering never changes the selected record.
	h.press("j", "enter")
	if m.sub == nil || m.sub.ID != 2 || !strings.Contains(h.view(), "Submission #2") {
		t.Fatal("narrow history lost submission identity")
	}
}

func TestSingleProblemHistoryColumns(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	p := h.m.selected()
	addSubs(t, h.m, "", p.Ref(), p.Ref())
	h.press("v", "h")
	header := strings.Split(h.view(), "\n")[3]
	if !strings.HasPrefix(header, "  SUB #") || strings.Contains(header, "PROBLEM") || strings.Contains(header, " ID ") {
		t.Fatalf("single-problem history has redundant problem metadata: %s", header)
	}
	h.press("j", "enter")
	if h.m.sub == nil || h.m.sub.ID != 2 {
		t.Fatal("submission selection changed")
	}
}

func TestHistoryColumnsRefreshAndEmpty(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	m := h.m
	h.press("h")
	for _, width := range []int{30, 40, 60, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
		if !strings.Contains(h.view(), "No submissions yet") {
			t.Fatal("empty history lost its hint")
		}
	}
	// Resolve the same slug against its own pack, and re-read IDs from the
	// loaded metadata after a refresh rather than caching them in history.
	addSubs(t, m, "", "regression/two-sum")
	m.loadHistory()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	if !strings.HasPrefix(strings.Split(h.view(), "\n")[4], "▌ 1 ") {
		t.Fatal("problem ID missing")
	}
	p := m.commandProblem("regression/two-sum")
	p.ID = "题 001"
	if !strings.HasPrefix(strings.Split(h.view(), "\n")[4], "▌ 题 001 ") {
		t.Fatal("updated Unicode ID missing")
	}
	if err := m.reload(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.Split(h.view(), "\n")[4], "▌ 1 ") {
		t.Fatal("ID not refreshed from problem metadata")
	}
}
