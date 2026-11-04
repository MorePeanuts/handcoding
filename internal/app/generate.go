package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/lang/python"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/version"
)

// Materialize builds the generated cases in sel that have no input yet: their
// inputs with generator.py, and their expected outputs with a reference
// solution, in l's language when the problem has one (nil: any). Every
// generated case of the problem is built at once and kept in the cache
// (.hc/cache/cases/), which later runs read. A case hc cannot build is a
// *runner.GeneratedCaseError.
func (a *App) Materialize(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, o RunOptions) error {
	if !slices.ContainsFunc(sel, unbuilt) {
		return nil
	}
	ref, err := a.generatedRef(p, l)
	if err != nil {
		return err
	}
	cases, err := p.LoadCases()
	if err != nil {
		return err
	}
	var gen []runner.Selected
	for i, c := range cases {
		if c.Generated() {
			gen = append(gen, runner.Selected{Index: i + 1, Case: c})
		}
	}
	key, err := a.cacheKey(p, ref, gen)
	if err != nil {
		return err
	}
	path := a.casesCachePath(p, ref)
	if !readCasesCache(path, key, gen) {
		if err := a.generateInputs(ctx, p, gen, true, o.Stderr); err != nil {
			return err
		}
		if err := a.expectGenerated(ctx, p, ref, gen, o.Stderr); err != nil {
			return err
		}
		writeCasesCache(path, key, gen)
	}
	built := make(map[int]problem.Case, len(gen))
	for _, g := range gen {
		built[g.Index] = g.Case
	}
	for i := range sel {
		c := &sel[i].Case
		if !unbuilt(sel[i]) {
			continue
		}
		b, ok := built[sel[i].Index]
		if !ok || b.Generate.String() != c.Generate.String() || b.Digest != c.Digest {
			return fmt.Errorf("case %d changed in %s while it was being built; run again", sel[i].Index, problem.CasesFile)
		}
		c.Input, c.Expected = b.Input, b.Expected
	}
	return nil
}

func unbuilt(s runner.Selected) bool { return s.Case.Generated() && s.Case.Input == nil }

// BuildCase builds generated cases that are not in cases.jsonl, such as a
// generator call given to hc test: their inputs with generator.py and, when
// p has a reference solution, their expected outputs, with the reference in
// l when there is one. Nothing is cached.
func (a *App) BuildCase(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, o RunOptions) error {
	err := a.generateInputs(ctx, p, sel, false, o.Stderr)
	if err == nil && len(a.ReferenceLangs(p)) > 0 {
		var ref lang.Language
		if ref, err = a.generatedRef(p, l); err == nil {
			err = a.expectGenerated(ctx, p, ref, sel, o.Stderr)
		}
	}
	if ge, ok := errors.AsType[*runner.GeneratedCaseError](err); ok {
		ge.Custom = true
	}
	return err
}

// generatedRef is the reference solution that computes the expected outputs
// of p's generated cases: the one in l when there is one, else another.
// References run only on toolchains the protocol supports, so one whose
// toolchain is too old (which only warns for the user's solution) gives way
// to the next.
func (a *App) generatedRef(p *problem.Problem, l lang.Language) (lang.Language, error) {
	refs := a.ReferenceLangs(p)
	if len(refs) == 0 {
		return nil, fmt.Errorf("%s has generated cases, whose expected outputs come from a reference solution, but it has none in %s/", p.Slug, problem.ReferenceDir)
	}
	if l != nil {
		if i := slices.IndexFunc(refs, func(r lang.Language) bool { return r.ID() == l.ID() }); i > 0 {
			refs = append([]lang.Language{refs[i]}, slices.Delete(slices.Clone(refs), i, i+1)...)
		}
	}
	for _, r := range refs {
		tc, err := r.Toolchain()
		if err == nil && tc.Err() == nil {
			return r, nil
		}
	}
	// RunReference reports why the first cannot run.
	return refs[0], nil
}

// GenerateInputs builds the inputs of the generated cases in sel with
// generator.py, as they are now: unlike a run, it neither checks them
// against their digests nor computes expected outputs, so that it shows
// what a changed generator.py makes before hc cases build.
func (a *App) GenerateInputs(ctx context.Context, p *problem.Problem, sel []runner.Selected, w io.Writer) error {
	return a.generateInputs(ctx, p, sel, false, w)
}

// generator returns what runs the problem's generator.py: the workspace's
// Python, whatever the solution's language, which must be a supported one
// since generators are part of the problem.
func (a *App) generator(ctx context.Context, p *problem.Problem, w io.Writer) (lang.Generator, error) {
	l, err := a.Langs.Get("python")
	if err != nil {
		return nil, err
	}
	g, ok := l.(lang.Generator)
	if !ok {
		return nil, errors.New("python cannot run generators")
	}
	needs := fmt.Sprintf("%s has cases built by %s, which needs Python %s or later", p.Slug, problem.GeneratorFile, python.MinVersion)
	if err := l.Setup(ctx, w); err != nil {
		return nil, fmt.Errorf("%s: %w", needs, err)
	}
	tc, err := l.Toolchain()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", needs, err)
	}
	if err := tc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", needs, err)
	}
	return g, nil
}

// generateInputs builds the inputs of the generated cases in sel that have
// none. With checkDigest, each must match its case's digest.
func (a *App) generateInputs(ctx context.Context, p *problem.Problem, sel []runner.Selected, checkDigest bool, w io.Writer) error {
	if !slices.ContainsFunc(sel, unbuilt) {
		return nil
	}
	if w == nil {
		w = io.Discard
	}
	g, err := a.generator(ctx, p, w)
	if err != nil {
		return err
	}
	return runner.Generate(ctx, p, sel, runner.GenerateOptions{
		Generator: g, Timeout: a.WS.Config.TimeoutDuration(), Name: a.WS.Rel(p.GeneratorPath()), CheckDigest: checkDigest,
	})
}

// expectGenerated sets the expected outputs of the generated cases in sel
// that have none, to the outputs of the reference in l. A reference that
// fails on one is a *runner.GeneratedCaseError.
func (a *App) expectGenerated(ctx context.Context, p *problem.Problem, l lang.Language, sel []runner.Selected, w io.Writer) error {
	var todo []runner.Selected
	var pos []int
	for i, s := range sel {
		if s.Case.Generated() && !s.Case.HasExpected() {
			todo = append(todo, s)
			pos = append(pos, i)
		}
	}
	if len(todo) == 0 {
		return nil
	}
	if w == nil {
		w = io.Discard
	}
	rep, _, err := a.RunReference(ctx, p, l.ID(), todo, RunOptions{Stderr: w})
	if err != nil {
		return err
	}
	file, _ := p.Reference(l.SolutionFile())
	name := a.WS.Rel(file)
	if rep.Verdict == runner.CompileError {
		return &runner.GeneratedCaseError{Path: name, Case: todo[0].Index, Reason: "the reference solution does not compile", Detail: rep.CompileError}
	}
	for j, cr := range rep.Cases {
		if cr.Verdict != runner.NoExpected {
			return &runner.GeneratedCaseError{Path: name, Case: cr.Index, Reason: "the reference solution: " + string(cr.Verdict), Detail: cr.Error}
		}
		sel[pos[j]].Case.Expected = cr.Output
	}
	return nil
}

// casesCachePath is the cache of p's generated cases built with the
// reference in l.
func (a *App) casesCachePath(p *problem.Problem, l lang.Language) string {
	return filepath.Join(a.WS.StatePath(), "cache", "cases", p.Pack, p.Slug, l.ID()+".json")
}

// cacheKey identifies what built the generated cases: hc, problem.yaml,
// generator.py, the reference in l and the generated cases' lines.
func (a *App) cacheKey(p *problem.Problem, l lang.Language, gen []runner.Selected) (string, error) {
	h := sha256.New()
	part(h, []byte(version.String()))
	ref, _ := p.Reference(l.SolutionFile())
	for _, path := range []string{filepath.Join(p.Dir, problem.MetaFile), p.GeneratorPath(), ref} {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		part(h, data)
	}
	for _, g := range gen {
		line, err := problem.MarshalCase(g.Case)
		if err != nil {
			return "", err
		}
		part(h, line)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// part writes b to h with its length, so that parts cannot run together.
func part(h hash.Hash, b []byte) {
	fmt.Fprintf(h, "%d:", len(b))
	h.Write(b)
}

type casesCache struct {
	Key   string       `json:"key"`
	Cases []cachedCase `json:"cases"`
}

type cachedCase struct {
	Input    map[string]json.RawMessage `json:"input"`
	Expected json.RawMessage            `json:"expected"`
}

// readCasesCache sets the inputs and expected outputs of gen from the cache
// file at path, if it holds them for key.
func readCasesCache(path, key string, gen []runner.Selected) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var c casesCache
	if json.Unmarshal(data, &c) != nil || c.Key != key || len(c.Cases) != len(gen) {
		return false
	}
	for _, cc := range c.Cases {
		if cc.Input == nil || cc.Expected == nil {
			return false
		}
	}
	for i, cc := range c.Cases {
		gen[i].Case.Input, gen[i].Case.Expected = cc.Input, cc.Expected
	}
	return true
}

// writeCasesCache writes the built cases gen to the cache file at path, for
// key. Like the other caches, it is best effort: a failure only costs
// building the cases again.
func writeCasesCache(path, key string, gen []runner.Selected) {
	c := casesCache{Key: key, Cases: make([]cachedCase, len(gen))}
	for i, g := range gen {
		c.Cases[i] = cachedCase{Input: g.Case.Input, Expected: g.Case.Expected}
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}
