package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// LeetCodeSessionsPath is the file holding the LeetCode sessions hc signs in
// with, one per site: in the user's configuration directory, shared by every
// workspace and never inside one, so that it cannot end up in a pack.
func LeetCodeSessionsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hc", "leetcode.json"), nil
}

// leetCodeSessions are the sessions of the file, by site name.
type leetCodeSessions map[string]leetcode.Session

func loadLeetCodeSessions() (leetCodeSessions, string, error) {
	path, err := LeetCodeSessionsPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return leetCodeSessions{}, path, nil
	}
	if err != nil {
		return nil, path, err
	}
	s := leetCodeSessions{}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, path, fmt.Errorf("%s is broken (%w); run hc leetcode logout, then log in again", path, err)
	}
	return s, path, nil
}

// save writes the sessions readable by the user only, atomically.
func (s leetCodeSessions) save(path string) error {
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

// LeetCodeSession returns the session saved for site.
func LeetCodeSession(site leetcode.Site) (*leetcode.Session, error) {
	s, _, err := loadLeetCodeSessions()
	if err != nil {
		return nil, err
	}
	sess, ok := s[site.Name]
	if !ok {
		return nil, fmt.Errorf("not logged in to leetcode.%s; run `hc leetcode login --site %s`", site.Name, site.Name)
	}
	return &sess, nil
}

// LeetCodeLogin checks that s signs in to c's site and saves it for the
// site. It returns the user's name and where the session is kept. A session
// LeetCode does not accept is not saved.
func LeetCodeLogin(ctx context.Context, c *leetcode.Client, s leetcode.Session) (user, path string, err error) {
	s.Token, s.CSRF = strings.TrimSpace(s.Token), strings.TrimSpace(s.CSRF)
	if s.Token == "" || s.CSRF == "" {
		return "", "", errors.New("both LEETCODE_SESSION and csrftoken are needed")
	}
	c.Session = &s
	user, err = c.Username(ctx)
	switch {
	case errors.Is(err, leetcode.ErrNotSignedIn):
		return "", "", fmt.Errorf("leetcode.%s does not accept this session: copy LEETCODE_SESSION and csrftoken again from a browser signed in to it", c.Site.Name)
	case err != nil:
		return "", "", err
	}
	all, path, err := loadLeetCodeSessions()
	if err != nil {
		return "", "", err
	}
	all[c.Site.Name] = s
	return user, path, all.save(path)
}

// LeetCodeLogout forgets the session of site, reporting whether there was
// one.
func LeetCodeLogout(site leetcode.Site) (bool, error) {
	all, path, err := loadLeetCodeSessions()
	if err != nil {
		// A broken file is removed whole.
		if path == "" {
			return false, err
		}
		return true, os.Remove(path)
	}
	if _, ok := all[site.Name]; !ok {
		return false, nil
	}
	delete(all, site.Name)
	if len(all) == 0 {
		return true, os.Remove(path)
	}
	return true, all.save(path)
}

// LeetCodeTarget is where a problem is on LeetCode.
type LeetCodeTarget struct {
	Site leetcode.Site
	Slug string
}

// LeetCodeProblem finds p on LeetCode from its source in problem.yaml: the
// site and slug of source.url, else source.site and p's slug. site, when
// set (com or cn), overrides the site. A problem that does not come from
// LeetCode is refused.
func LeetCodeProblem(p *problem.Problem, site string) (LeetCodeTarget, error) {
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
	if site != "" {
		s, err := leetcode.SiteByName(site)
		if err != nil {
			return t, err
		}
		t.Site = s
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
func (a *App) leetCodeCode(ctx context.Context, c *leetcode.Client, p *problem.Problem, l lang.Language, slug string) (leetcode.Code, *leetcode.Question, error) {
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
	q, err := c.QuestionMeta(ctx, slug)
	if err != nil {
		return leetcode.Code{}, nil, err
	}
	return leetcode.Code{Slug: q.TitleSlug, QuestionID: q.QuestionID, Lang: langSlug, Code: code}, q, nil
}

// LeetCodeTest runs the user's solution of p in l on LeetCode, signed in by
// c, on input (LeetCode's format), or on the question's example testcases
// when input is empty.
func (a *App) LeetCodeTest(ctx context.Context, c *leetcode.Client, p *problem.Problem, l lang.Language, slug, input string) (*LeetCodeRun, error) {
	code, q, err := a.leetCodeCode(ctx, c, p, l, slug)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input) == "" {
		input = q.ExampleTestcases
	}
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("%s has no example testcases on leetcode.%s; give an input with --input", slug, c.Site.Name)
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
func (a *App) LeetCodeSubmit(ctx context.Context, c *leetcode.Client, p *problem.Problem, l lang.Language, slug string) (*LeetCodeRun, error) {
	code, _, err := a.leetCodeCode(ctx, c, p, l, slug)
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
