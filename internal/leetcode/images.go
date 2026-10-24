package leetcode

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// maxImageSize bounds a downloaded statement image.
const maxImageSize = 10 << 20

// DownloadImages fetches the images of the statement into c.Assets and
// points the description at the local copies, so the statement renders
// offline. An image that cannot be downloaded keeps its URL and adds a
// warning.
func (cl *Client) DownloadImages(ctx context.Context, c *Converted) {
	base, _ := url.Parse(cl.Site.Base)
	local := map[string]string{} // URL -> assets/<name>
	c.Description = problem.ReplaceImages(c.Description, func(alt, src string) string {
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
			rel = path.Join(problem.AssetsDir, c.assetName(u.Path))
			if c.Assets == nil {
				c.Assets = map[string][]byte{}
			}
			c.Assets[rel] = data
			local[u.String()] = rel
		}
		return "![" + alt + "](" + rel + ")"
	})
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

// assetName picks a file name under assets/ for an image URL path.
func (c *Converted) assetName(urlPath string) string {
	name := strings.Trim(unsafeNameRE.ReplaceAllString(path.Base(urlPath), "-"), "-.")
	if name == "" {
		name = "image"
	}
	candidate := name
	for i := 2; c.Assets[path.Join(problem.AssetsDir, candidate)] != nil; i++ {
		candidate = fmt.Sprintf("%d-%s", i, name)
	}
	return candidate
}
