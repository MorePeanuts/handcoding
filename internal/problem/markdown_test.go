package problem

import "testing"

func TestReplaceImages(t *testing.T) {
	md := "a ![one](assets/1.png) b\n![](https://x.test/2.jpg \"title\")\n![x](<assets/3.gif>)\n[not](an image)"
	var got []string
	out := ReplaceImages(md, func(alt, src string) string {
		got = append(got, alt+"|"+src)
		return "<" + src + ">"
	})
	want := []string{"one|assets/1.png", "|https://x.test/2.jpg", "x|assets/3.gif"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("image %d = %q, want %q", i, got[i], want[i])
		}
	}
	if wantOut := "a <assets/1.png> b\n<https://x.test/2.jpg>\n<assets/3.gif>\n[not](an image)"; out != wantOut {
		t.Errorf("out = %q, want %q", out, wantOut)
	}
}

func TestReplaceImagesSkipsCode(t *testing.T) {
	md := "```\nassets/1.png  written as ![](assets/1.png)\n```\n![](assets/2.png)\n"
	var got []string
	out := ReplaceImages(md, func(_, src string) string {
		got = append(got, src)
		return "IMG"
	})
	if len(got) != 1 || got[0] != "assets/2.png" {
		t.Errorf("replaced %q, want only assets/2.png", got)
	}
	if want := "```\nassets/1.png  written as ![](assets/1.png)\n```\nIMG\n"; out != want {
		t.Errorf("out = %q, want %q", out, want)
	}
}
