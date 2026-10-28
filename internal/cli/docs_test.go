package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/docs"
	"github.com/MorePeanuts/handcoding/internal/markdown"
)

// Skills cite the protocol by section id, and the document links to its own
// sections the same way: every internal link must name a section.
func TestProtocolLinksResolve(t *testing.T) {
	sections := protocolSections()
	links := regexp.MustCompile(`\]\(#([^)]+)\)`).FindAllStringSubmatch(docs.Protocol, -1)
	if len(links) == 0 {
		t.Fatal("no internal links found")
	}
	for _, l := range links {
		if _, ok := markdown.FindSection(sections, l[1]); !ok {
			t.Errorf("link #%s names no section of docs/protocol.md", l[1])
		}
	}
}

func TestProtocolSection(t *testing.T) {
	sections := protocolSections()
	text, err := sectionText(sections, "checkers")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "### Checkers\n") || strings.Contains(text, "\n### ") {
		t.Errorf("checkers section:\n%s", text)
	}
	if _, err := sectionText(sections, "handcoding-protocols"); err == nil {
		t.Error("the document title is a section")
	}
	_, err = sectionText(sections, "checker")
	if err == nil || !strings.Contains(err.Error(), "did you mean checkers?") || !strings.Contains(err.Error(), "--list") {
		t.Errorf("unknown section error = %v", err)
	}
}
