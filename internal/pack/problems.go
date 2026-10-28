package pack

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

// LoadError is a problem directory whose problem.yaml could not be loaded.
type LoadError struct {
	Dir string
	Err error
}

func (e *LoadError) Error() string { return e.Err.Error() }
func (e *LoadError) Unwrap() error { return e.Err }

// LoadProblems loads the problems in dir, the problems/ directory of the pack
// called name, sorted by numeric id, then slug. Every directory in it that
// holds a problem.yaml is a problem; one that fails to load is returned as a
// *LoadError instead. A missing dir holds no problems.
func LoadProblems(dir, name string) ([]*problem.Problem, []*LoadError, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var (
		probs  []*problem.Problem
		failed []*LoadError
	)
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		// Stat follows symbolic links, so a linked problem directory counts.
		if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
			continue
		}
		p, err := problem.Load(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failed = append(failed, &LoadError{Dir: path, Err: err})
			continue
		}
		p.Pack = name
		probs = append(probs, p)
	}
	slices.SortFunc(probs, func(a, b *problem.Problem) int {
		an, aerr := strconv.Atoi(a.ID)
		bn, berr := strconv.Atoi(b.ID)
		switch {
		case aerr == nil && berr == nil && an != bn:
			return cmp.Compare(an, bn)
		case aerr == nil && berr != nil:
			return -1
		case aerr != nil && berr == nil:
			return 1
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	return probs, failed, nil
}

// IdentityErrors reports how p conflicts with probs, the problems of its pack
// (docs/protocol.md, "Identity"): solutions and history are keyed by the
// directory name, so it must be p.DirName(), and slug and id (when present)
// must be unique within the pack.
func IdentityErrors(p *problem.Problem, probs []*problem.Problem) []error {
	var errs []error
	if base := filepath.Base(p.Dir); base != p.DirName() {
		errs = append(errs, fmt.Errorf("directory %s must be named %s, after id and slug", base, p.DirName()))
	}
	for _, o := range probs {
		if o.Dir == p.Dir {
			continue
		}
		if o.Slug == p.Slug {
			errs = append(errs, fmt.Errorf("slug %s is also used by %s", p.Slug, filepath.Base(o.Dir)))
		}
		if p.ID != "" && o.ID == p.ID {
			errs = append(errs, fmt.Errorf("id %s is also used by %s", p.ID, o.Slug))
		}
	}
	return errs
}
