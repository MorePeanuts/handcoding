package leetcode

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// maxImageSize bounds a downloaded statement image.
const maxImageSize = 10 << 20

// DownloadImages fetches the images of the statements, in every locale,
// into c.Assets and points the statements at the local copies, so they
// render offline. The statements share assets/: an image already there
// under its name is not stored twice, since the two sites serve the same
// image from different URLs. An image that cannot be downloaded keeps its
// URL and adds a warning.
func (cl *Client) DownloadImages(ctx context.Context, c *Converted) {
	base, _ := url.Parse(cl.Site.Base)
	local := map[string]string{} // URL -> assets/<name>
	localize := func(md string) string {
		return problem.ReplaceImages(md, func(alt, src string) string {
			keep := "![" + alt + "](" + src + ")"
			u, err := url.Parse(src)
			if err != nil {
				return keep
			}
			u = base.ResolveReference(u)
			if u.Scheme != "http" && u.Scheme != "https" {
				return keep
			}
			rel, ok := local[u.String()]
			if !ok {
				data, err := cl.image(ctx, u.String())
				if err != nil {
					c.Warnings = append(c.Warnings, fmt.Sprintf("image %s not downloaded (%v); the statement links to it instead", u, err))
					return keep
				}
				rel = c.store(u.Path, data)
				local[u.String()] = rel
			}
			return "![" + alt + "](" + rel + ")"
		})
	}
	c.Description = localize(c.Description)
	for _, l := range slices.Sorted(maps.Keys(c.Translations)) {
		c.Translations[l] = localize(c.Translations[l])
	}
}

func (cl *Client) image(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := cl.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageSize {
		return nil, fmt.Errorf("larger than %d MiB", maxImageSize>>20)
	}
	if ct := http.DetectContentType(data); !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("not an image (%s)", ct)
	}
	return data, nil
}

var unsafeNameRE = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// store puts data, the image at urlPath, into c.Assets, and returns its
// path in the problem directory. An identical file among the assets or the
// existing files is reused; else it is named after urlPath, and picks
// another name when that one is taken.
func (c *Converted) store(urlPath string, data []byte) string {
	for _, files := range []map[string][]byte{c.Assets, c.Existing} {
		for _, rel := range slices.Sorted(maps.Keys(files)) {
			if bytes.Equal(files[rel], data) {
				return rel
			}
		}
	}
	name := strings.Trim(unsafeNameRE.ReplaceAllString(path.Base(urlPath), "-"), "-.")
	if name == "" {
		name = "image"
	}
	candidate := name
	for i := 2; ; i++ {
		rel := path.Join(problem.AssetsDir, candidate)
		_, taken := c.Assets[rel]
		if _, ok := c.Existing[rel]; ok {
			taken = true
		}
		if !taken {
			if c.Assets == nil {
				c.Assets = map[string][]byte{}
			}
			c.Assets[rel] = data
			return rel
		}
		candidate = fmt.Sprintf("%d-%s", i, name)
	}
}
