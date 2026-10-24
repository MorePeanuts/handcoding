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

func TestDownloadImages(t *testing.T) {
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uploads/a/ex1.png", "/uploads/b/ex1.png":
			_, _ = w.Write(pngData.Bytes())
		case "/page.png":
			_, _ = w.Write([]byte("<html>not an image</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Converted{Description: strings.Join([]string{
		"![](" + srv.URL + "/uploads/a/ex1.png)",
		"![again](" + srv.URL + "/uploads/a/ex1.png)",
		"![](/uploads/b/ex1.png)", // relative to the site, same file name
		"![](" + srv.URL + "/missing.png)",
		"![](" + srv.URL + "/page.png)",
	}, "\n")}
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
	if len(c.Assets) != 2 || !bytes.Equal(c.Assets["assets/ex1.png"], pngData.Bytes()) || c.Assets["assets/2-ex1.png"] == nil {
		t.Errorf("assets: %v", len(c.Assets))
	}
	if len(c.Warnings) != 2 {
		t.Errorf("warnings: %q", c.Warnings)
	}
}
