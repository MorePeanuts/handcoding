// Package leetcode fetches problems from leetcode.com / leetcode.cn and
// converts them into the handcoding problem format.
package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Site is a LeetCode site.
type Site struct {
	Name string // com or cn
	Base string
}

// Sites supported by the client.
var (
	SiteCom = Site{Name: "com", Base: "https://leetcode.com"}
	SiteCN  = Site{Name: "cn", Base: "https://leetcode.cn"}
)

// SiteByName returns a site by "com" or "cn".
func SiteByName(name string) (Site, error) {
	switch strings.TrimPrefix(strings.ToLower(name), "leetcode.") {
	case "", "com", "us":
		return SiteCom, nil
	case "cn", "zh":
		return SiteCN, nil
	}
	return Site{}, fmt.Errorf("unknown LeetCode site %q (com or cn)", name)
}

// Client talks to LeetCode's public GraphQL API. No login is needed for free
// problems.
type Client struct {
	Site     Site
	HTTP     *http.Client
	CacheDir string // caches the problem list used to resolve IDs
}

// NewClient creates a client.
func NewClient(site Site, cacheDir string) *Client {
	return &Client{Site: site, HTTP: &http.Client{Timeout: 30 * time.Second}, CacheDir: cacheDir}
}

// Question is the subset of LeetCode's question object hc uses.
type Question struct {
	QuestionID         string `json:"questionId"`
	QuestionFrontendID string `json:"questionFrontendId"`
	Title              string `json:"title"`
	TitleSlug          string `json:"titleSlug"`
	TranslatedTitle    string `json:"translatedTitle"`
	Content            string `json:"content"`
	TranslatedContent  string `json:"translatedContent"`
	IsPaidOnly         bool   `json:"isPaidOnly"`
	Difficulty         string `json:"difficulty"`
	TopicTags          []struct {
		Slug string `json:"slug"`
	} `json:"topicTags"`
	CodeSnippets []struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	} `json:"codeSnippets"`
	MetaData         string `json:"metaData"`
	ExampleTestcases string `json:"exampleTestcases"`
}

const questionQuery = `query questionData($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    questionId questionFrontendId title titleSlug content
    translatedTitle translatedContent isPaidOnly difficulty
    topicTags { slug }
    codeSnippets { langSlug code }
    metaData exampleTestcases
  }
}`

// ErrNotFound is returned for unknown problems.
var ErrNotFound = errors.New("problem not found on LeetCode")

// Question fetches a question by slug.
func (c *Client) Question(ctx context.Context, slug string) (*Question, error) {
	body, _ := json.Marshal(map[string]any{
		"operationName": "questionData",
		"query":         questionQuery,
		"variables":     map[string]string{"titleSlug": slug},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Site.Base+"/graphql/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", c.Site.Base+"/problems/"+slug+"/")
	req.Header.Set("User-Agent", "handcoding/1 (+https://github.com/MorePeanuts/handcoding)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("leetcode: HTTP %d: %s", resp.StatusCode, snippet(data))
	}
	var out struct {
		Data struct {
			Question *Question `json:"question"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("leetcode: decode response: %w (%s)", err, snippet(data))
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("leetcode: %s", out.Errors[0].Message)
	}
	if out.Data.Question == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, slug)
	}
	return out.Data.Question, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

type listEntry struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
}

// ResolveSlug turns a frontend ID (e.g. "146") into a slug; anything that is
// not a number is returned as is.
func (c *Client) ResolveSlug(ctx context.Context, ref string) (string, error) {
	if _, err := strconv.Atoi(ref); err != nil {
		return strings.TrimSuffix(strings.TrimPrefix(ref, c.Site.Base+"/problems/"), "/"), nil
	}
	entries, err := c.problemList(ctx)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.ID == ref {
			return e.Slug, nil
		}
	}
	return "", fmt.Errorf("%w: id %s", ErrNotFound, ref)
}

func (c *Client) problemList(ctx context.Context) ([]listEntry, error) {
	cache := ""
	if c.CacheDir != "" {
		cache = filepath.Join(c.CacheDir, "leetcode-"+c.Site.Name+"-problems.json")
		if st, err := os.Stat(cache); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
			if data, err := os.ReadFile(cache); err == nil {
				var entries []listEntry
				if json.Unmarshal(data, &entries) == nil && len(entries) > 0 {
					return entries, nil
				}
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Site.Base+"/api/problems/all/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "handcoding/1 (+https://github.com/MorePeanuts/handcoding)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("leetcode: problem list: HTTP %d", resp.StatusCode)
	}
	var all struct {
		Pairs []struct {
			Stat struct {
				FrontendID any    `json:"frontend_question_id"`
				Slug       string `json:"question__title_slug"`
			} `json:"stat"`
		} `json:"stat_status_pairs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("leetcode: problem list: %w", err)
	}
	entries := make([]listEntry, 0, len(all.Pairs))
	for _, p := range all.Pairs {
		id := fmt.Sprint(p.Stat.FrontendID)
		if f, ok := p.Stat.FrontendID.(float64); ok {
			id = strconv.Itoa(int(f))
		}
		entries = append(entries, listEntry{ID: id, Slug: p.Stat.Slug})
	}
	if cache != "" {
		if data, err := json.Marshal(entries); err == nil {
			_ = os.MkdirAll(c.CacheDir, 0o755)
			_ = os.WriteFile(cache, data, 0o644)
		}
	}
	return entries, nil
}
