package leetcode

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pngOfSize(t *testing.T, n int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, n, n))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDownloadImages(t *testing.T) {
	small, big, old := pngOfSize(t, 4), pngOfSize(t, 8), pngOfSize(t, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads/a/ex1.png", "/cn/copy.png":
			_, _ = w.Write(small)
		case "/uploads/b/ex1.png":
			_, _ = w.Write(big)
		case "/cn/old.png":
			_, _ = w.Write(old)
		case "/page.png":
			_, _ = w.Write([]byte("<html>not an image</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Converted{
		Description: strings.Join([]string{
			"![](" + srv.URL + "/uploads/a/ex1.png)",
			"![again](" + srv.URL + "/uploads/a/ex1.png)",
			"![](/uploads/b/ex1.png)", // relative to the site, same file name
			"![](" + srv.URL + "/missing.png)",
			"![](" + srv.URL + "/page.png)",
		}, "\n"),
		// The translation shares assets/: the same image from another URL
		// is stored once, and so is one that is there already.
		Translations: map[string]string{"zh-CN": "![](/cn/copy.png)\n![](/cn/old.png)"},
		Existing:     map[string][]byte{"assets/ex1-old.png": old},
	}
	cl := NewClient(Site{Name: "test", Base: srv.URL}, "")
	cl.DownloadImages(context.Background(), c)

	want := strings.Join([]string{
		"![](assets/ex1.png)",
		"![again](assets/ex1.png)",
		"![](assets/2-ex1.png)",
		"![](" + srv.URL + "/missing.png)",
		"![](" + srv.URL + "/page.png)",
	}, "\n")
	if c.Description != want {
		t.Errorf("description:\n%s\nwant:\n%s", c.Description, want)
	}
	if got, want := c.Translations["zh-CN"], "![](assets/ex1.png)\n![](assets/ex1-old.png)"; got != want {
		t.Errorf("translation:\n%s\nwant:\n%s", got, want)
	}
	if len(c.Assets) != 2 || !bytes.Equal(c.Assets["assets/ex1.png"], small) || !bytes.Equal(c.Assets["assets/2-ex1.png"], big) {
		t.Errorf("assets: %v", len(c.Assets))
	}
	if len(c.Warnings) != 2 {
		t.Errorf("warnings: %q", c.Warnings)
	}
}
