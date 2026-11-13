package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// LeetCodeSessionsPath is global to the user, shared by every workspace.
func LeetCodeSessionsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hc", "leetcode.json"), nil
}

// LeetCodeAccount is the single account used for online runs and submissions.
type LeetCodeAccount struct {
	Site    leetcode.Site
	User    string
	Session leetcode.Session
}

type savedLeetCodeAccount struct {
	Site    string           `json:"site"`
	User    string           `json:"user"`
	Session leetcode.Session `json:"credentials"`
}

func loadLeetCodeAccount() (*LeetCodeAccount, string, error) {
	path, err := LeetCodeSessionsPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, err
	}
	var saved savedLeetCodeAccount
	if err := json.Unmarshal(data, &saved); err != nil ||
		(saved.Site != "com" && saved.Site != "cn") || strings.TrimSpace(saved.Session.Token) == "" {
		return nil, path, fmt.Errorf("%s contains invalid or older login state; run hc leetcode login --site cn or --site com again", path)
	}
	site, err := leetcode.SiteByName(saved.Site)
	if err != nil {
		return nil, path, err
	}
	return &LeetCodeAccount{Site: site, User: saved.User, Session: saved.Session}, path, nil
}

// save replaces the single login atomically, with user-only permissions.
func (s savedLeetCodeAccount) save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".leetcode-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// LeetCodeSession loads the current account, never choosing a site from a pack.
func LeetCodeSession() (*LeetCodeAccount, error) {
	a, _, err := loadLeetCodeAccount()
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, fmt.Errorf("not logged in to LeetCode; run hc leetcode login")
	}
	return a, nil
}

// LeetCodeLogin verifies a new login before replacing the previous account.
// Invalid, cancelled or failed logins leave the previous login intact.
func LeetCodeLogin(ctx context.Context, c *leetcode.Client, s leetcode.Session) (user, path string, err error) {
	if c.Site.Name != "cn" && c.Site.Name != "com" {
		return "", "", fmt.Errorf("unsupported LeetCode site %q", c.Site.Name)
	}
	s.Token = strings.TrimSpace(s.Token)
	if s.Token == "" {
		return "", "", fmt.Errorf("LEETCODE_SESSION is needed")
	}
	c.Session = &s
	user, err = c.Username(ctx)
	switch {
	case errors.Is(err, leetcode.ErrNotSignedIn):
		return "", "", fmt.Errorf("leetcode.%s does not accept this session: copy LEETCODE_SESSION again from a browser signed in to it", c.Site.Name)
	case err != nil:
		return "", "", err
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	path, err = LeetCodeSessionsPath()
	if err != nil {
		return "", "", err
	}
	return user, path, (savedLeetCodeAccount{Site: c.Site.Name, User: user, Session: s}).save(path)
}

// LeetCodeSiteStatus describes the current login, including an expired session.
type LeetCodeSiteStatus struct {
	Site leetcode.Site
	User string
	Err  error
}

// LeetCodeStatus checks only the current account. A nil status means signed out.
func LeetCodeStatus(ctx context.Context) (*LeetCodeSiteStatus, string, error) {
	a, path, err := loadLeetCodeAccount()
	if err != nil || a == nil {
		return nil, path, err
	}
	c := leetcode.NewClient(a.Site, "")
	c.Session = &a.Session
	user, err := c.Username(ctx)
	return &LeetCodeSiteStatus{Site: a.Site, User: user, Err: err}, path, nil
}

// LeetCodeBrowserLogin is a login through the browser, as LeetCode's VS Code
// extension logs in: LeetCode's authorize-login page, opened in a browser
// signed in to the site, redirects to a callback with the browser's cookies.
// The callback is a server on 127.0.0.1, at a path made for this login, so
// that nothing else can hand it a session.
type LeetCodeBrowserLogin struct {
	// URL is the page to open in the browser.
	URL string
	// Port is the port of 127.0.0.1 the callback listens on.
	Port int

	c      *leetcode.Client
	cancel context.CancelFunc
	path   string
	ln     net.Listener
	srv    *http.Server
	once   sync.Once
	done   chan browserLogin
}

type browserLogin struct {
	user, path string
	err        error
}

// StartLeetCodeBrowserLogin starts the callback server of a login to c's
// site, on port of 127.0.0.1 (0: any free port).
func StartLeetCodeBrowserLogin(c *leetcode.Client, port int) (*LeetCodeBrowserLogin, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("starting the login's callback on 127.0.0.1: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &LeetCodeBrowserLogin{
		c: c, cancel: cancel, path: "/callback/" + rand.Text(), ln: ln, done: make(chan browserLogin, 1),
	}
	b.Port = ln.Addr().(*net.TCPAddr).Port
	b.URL = c.Site.AuthorizeURL(ln.Addr().String() + b.path)
	b.srv = &http.Server{Handler: http.HandlerFunc(b.callback), ReadHeaderTimeout: 10 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() { _ = b.srv.Serve(ln) }()
	return b, nil
}

// callback takes the session from the cookies LeetCode's redirect carries,
// once, and tells the browser how the login went.
func (b *LeetCodeBrowserLogin) callback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != b.path {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	handled := false
	b.once.Do(func() {
		handled = true
		var res browserLogin
		s, err := leetcode.ParseSession(r.URL.Query().Get("cookie"))
		if err != nil {
			res.err = fmt.Errorf("leetcode.%s's authorization sent no session: %w", b.c.Site.Name, err)
		} else {
			res.user, res.path, res.err = LeetCodeLogin(r.Context(), b.c, s)
		}
		if res.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "hc could not log in: %v\n\nSee the terminal.\n", res.err)
		} else {
			fmt.Fprintf(w, "hc is logged in to leetcode.%s as %s. You can close this tab.\n", b.c.Site.Name, res.user)
		}
		b.done <- res
	})
	if !handled {
		fmt.Fprintln(w, "This login is over; see the terminal.")
	}
}

// Wait waits for the login, and returns the user's name and where the
// session is kept, as LeetCodeLogin does.
func (b *LeetCodeBrowserLogin) Wait(ctx context.Context) (user, path string, err error) {
	select {
	case res := <-b.done:
		return res.user, res.path, res.err
	case <-ctx.Done():
		b.cancel()
		return "", "", ctx.Err()
	}
}

// Close stops the callback server, after the reply to the browser is sent.
func (b *LeetCodeBrowserLogin) Close() error {
	b.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return b.srv.Shutdown(ctx)
}

// LeetCodeLogout clears hc's login, including malformed or older state.
// It does not invalidate the browser's session on LeetCode.
func LeetCodeLogout() (bool, error) {
	path, err := LeetCodeSessionsPath()
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// LeetCodeTarget identifies the original problem on its source site.
type LeetCodeTarget struct {
	Site leetcode.Site
	Slug string
}

// LeetCodeProblem resolves provenance only; the login chooses the destination.
// source.url takes precedence over source.site and the local slug.
func LeetCodeProblem(p *problem.Problem) (LeetCodeTarget, error) {
	var t LeetCodeTarget
	if s, slug, ok := leetcode.ParseURL(p.Source.URL); ok {
		t = LeetCodeTarget{Site: s, Slug: slug}
	} else if strings.HasPrefix(strings.ToLower(p.Source.Site), "leetcode.") {
		s, err := leetcode.SiteByName(p.Source.Site)
		if err != nil {
			return t, err
		}
		t = LeetCodeTarget{Site: s, Slug: p.Slug}
	} else {
		return t, fmt.Errorf("%s does not come from LeetCode (source in %s), so hc leetcode cannot run it there", p.Slug, problem.MetaFile)
	}
	return t, nil
}

// LeetCodeRun is the outcome of running or submitting a solution on
// LeetCode.
type LeetCodeRun struct {
	Lang lang.Language
	// Input is the testcases a run used, in LeetCode's format.
	Input  string
	Result *leetcode.Result
	// Case is the case a submission failed on, as a case of the problem,
	// when its input maps onto the problem's directly (LeetCodeCase).
	Case *problem.Case
}

// leetCodeCode reads the user's solution of p in l and turns it into
// LeetCode's code, through the driver.
func (a *App) leetCodeCode(ctx context.Context, c *leetcode.Client, p *problem.Problem, l lang.Language) (leetcode.Code, *leetcode.Question, error) {
	conv, ok := l.(lang.LeetCoder)
	if !ok {
		return leetcode.Code{}, nil, fmt.Errorf("hc cannot run %s solutions on LeetCode", l.Name())
	}
	path := a.SolutionPath(p, l)
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return leetcode.Code{}, nil, fmt.Errorf("no %s solution of %s yet; run `hc start %s -l %s`", l.Name(), p.Slug, a.WS.Ref(p), l.ID())
	}
	if err != nil {
		return leetcode.Code{}, nil, err
	}
	langSlug, code, err := conv.LeetCode(p, string(src))
	if err != nil {
		return leetcode.Code{}, nil, err
	}
	q, err := leetCodeQuestion(ctx, c, p)
	if err != nil {
		return leetcode.Code{}, nil, err
	}
	return leetcode.Code{Slug: q.TitleSlug, QuestionID: q.QuestionID, Lang: langSlug, Code: code}, q, nil
}

// leetCodeQuestion resolves the candidate on the signed-in site. For cross-site
// runs require a standard numbered question, matching canonical slug and ID,
// and identical signature metadata from both live sites. No title/ID fallback.
func leetCodeQuestion(ctx context.Context, c *leetcode.Client, p *problem.Problem) (*leetcode.Question, error) {
	source, err := LeetCodeProblem(p)
	if err != nil {
		return nil, err
	}
	target, err := c.QuestionMeta(ctx, source.Slug)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s on logged-in site leetcode.%s: %w", source.Slug, c.Site.Name, err)
	}
	if target.QuestionID == "" {
		return nil, fmt.Errorf("missing question ID for %s on leetcode.%s", source.Slug, c.Site.Name)
	}
	if source.Site.Name == c.Site.Name {
		return target, nil
	}
	// Use the same HTTP transport (timeouts/proxies), but never the target
	// session or cookie jar when contacting the source site.
	origin := leetcode.NewClient(source.Site, "")
	client := *c.HTTP
	client.Jar = nil
	origin.HTTP = &client
	original, err := origin.QuestionMeta(ctx, source.Slug)
	if err != nil {
		// A refusal of anonymous source metadata is not evidence that the
		// destination login expired; do not propagate its authentication sentinel.
		if errors.Is(err, leetcode.ErrBlocked) {
			return nil, fmt.Errorf("leetcode.%s refused source metadata; cannot verify cross-site mapping to leetcode.%s; try again or log in to the source site with hc leetcode login --site %s", source.Site.Name, c.Site.Name, source.Site.Name)
		}
		return nil, fmt.Errorf("cannot verify cross-site mapping from leetcode.%s to leetcode.%s: %w", source.Site.Name, c.Site.Name, err)
	}
	id, idErr := strconv.ParseUint(original.QuestionFrontendID, 10, 64)
	var sourceMeta, targetMeta map[string]any
	validSource := json.Unmarshal([]byte(original.MetaData), &sourceMeta) == nil && len(sourceMeta) > 0
	validTarget := json.Unmarshal([]byte(target.MetaData), &targetMeta) == nil && len(targetMeta) > 0
	if idErr != nil || id == 0 || strconv.FormatUint(id, 10) != original.QuestionFrontendID ||
		original.QuestionFrontendID != target.QuestionFrontendID ||
		original.TitleSlug != source.Slug || target.TitleSlug != source.Slug ||
		!validSource || !validTarget || !reflect.DeepEqual(sourceMeta, targetMeta) {
		return nil, fmt.Errorf("cannot verify cross-site mapping for %s from leetcode.%s to leetcode.%s (standard question ID, slug and signature must match); log in to the source site with hc leetcode login --site %s", source.Slug, source.Site.Name, c.Site.Name, source.Site.Name)
	}
	return target, nil
}

// LeetCodeTest runs the user's solution of p in l on LeetCode, signed in by
// c, on input (LeetCode's format), or on the question's example testcases
// when input is empty.
func (a *App) LeetCodeTest(ctx context.Context, c *leetcode.Client, p *problem.Problem, l lang.Language, input string) (*LeetCodeRun, error) {
	code, q, err := a.leetCodeCode(ctx, c, p, l)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input) == "" {
		input = q.ExampleTestcases
	}
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("%s has no example testcases on leetcode.%s; give an input with --input", q.TitleSlug, c.Site.Name)
	}
	id, err := c.Interpret(ctx, code, input)
	if err != nil {
		return nil, err
	}
	r, err := c.Wait(ctx, code.Slug, id)
	if err != nil {
		return nil, err
	}
	return &LeetCodeRun{Lang: l, Input: input, Result: r}, nil
}

// LeetCodeSubmit submits the user's solution of p in l to LeetCode, signed
// in by c. When it fails on a case whose input maps onto p's, Case holds it.
func (a *App) LeetCodeSubmit(ctx context.Context, c *leetcode.Client, p *problem.Problem, l lang.Language) (*LeetCodeRun, error) {
	code, _, err := a.leetCodeCode(ctx, c, p, l)
	if err != nil {
		return nil, err
	}
	id, err := c.Submit(ctx, code)
	if err != nil {
		return nil, err
	}
	r, err := c.Wait(ctx, code.Slug, id)
	if err != nil {
		return nil, err
	}
	run := &LeetCodeRun{Lang: l, Result: r}
	if !r.Accepted() && r.LastTestcase != "" {
		if c, ok := LeetCodeCase(p, string(r.LastTestcase), string(r.ExpectedOutput)); ok {
			run.Case = &c
		}
	}
	return run, nil
}

// LeetCodeCase turns a LeetCode testcase (one JSON value per line, a line
// per parameter) and its expected output into a case of p. Only a function
// problem whose parameters are LeetCode's, with no structure inputs and no
// judge-defined types (whose LeetCode form differs from hc's), maps this
// way; ok is false otherwise, or when the result is not a valid case.
// The expected output is left out with any_of, which needs every answer.
func LeetCodeCase(p *problem.Problem, input, expected string) (problem.Case, bool) {
	if !plainFunction(p) {
		return problem.Case{}, false
	}
	lines := strings.Split(strings.TrimRight(input, "\r\n"), "\n")
	f := p.Function
	if len(lines) != len(f.Params) {
		return problem.Case{}, false
	}
	c := problem.Case{Input: map[string]json.RawMessage{}}
	for i, prm := range f.Params {
		v := strings.TrimSpace(lines[i])
		if !json.Valid([]byte(v)) {
			return problem.Case{}, false
		}
		c.Input[prm.Name] = json.RawMessage(v)
	}
	if e := strings.TrimSpace(expected); e != "" && json.Valid([]byte(e)) && p.Judge.Compare != problem.CompareAnyOf {
		c.Expected = json.RawMessage(e)
	}
	if p.ValidateCase(c) != nil {
		c.Expected = nil
		if p.ValidateCase(c) != nil {
			return problem.Case{}, false
		}
	}
	return c, true
}

// plainFunction reports whether p is a function problem whose case inputs
// are LeetCode's testcases as they are: no structure inputs, no node
// returned by reference or copy, and no judge-defined types.
func plainFunction(p *problem.Problem) bool {
	f := p.Function
	if f == nil || len(f.Hidden) > 0 || f.ReturnsRef != "" || f.ReturnsCopy != "" {
		return false
	}
	for _, prm := range f.Params {
		if prm.Cycle != "" || prm.Join != nil || prm.Ref != "" || prm.Copy != "" {
			return false
		}
	}
	for _, t := range p.BaseTypes() {
		if problem.IsNodeVariant(t) || t == problem.TypeNestedInteger || t == problem.TypeEmployee {
			return false
		}
	}
	return true
}
