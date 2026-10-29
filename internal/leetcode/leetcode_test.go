package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// On leetcode.com an ID is resolved by searching for it, never by the
// problem list, which the site refuses; the search also returns questions
// whose title or slug merely contains the ID.
func TestResolveSlugCom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql/" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req struct {
			Variables struct {
				SearchKeywords string `json:"searchKeywords"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		data := map[string][]map[string]string{
			"2": {
				{"questionFrontendId": "650", "titleSlug": "2-keys-keyboard"},
				{"questionFrontendId": "2", "titleSlug": "add-two-numbers"},
			},
			"9999": {},
		}[req.Variables.SearchKeywords]
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"questionList": map[string]any{"data": data}}})
	}))
	defer srv.Close()
	c := NewClient(Site{Name: SiteCom.Name, Base: srv.URL}, "")
	for ref, want := range map[string]string{"2": "add-two-numbers", "two-sum": "two-sum"} {
		if got, err := c.ResolveSlug(context.Background(), ref); err != nil || got != want {
			t.Errorf("ResolveSlug(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
	if _, err := c.ResolveSlug(context.Background(), "9999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ResolveSlug(9999) = %v, want ErrNotFound", err)
	}
}
