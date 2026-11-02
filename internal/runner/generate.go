package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// GenerateItem is what generator.py gets for one generated case.
type GenerateItem struct {
	ID   int                        `json:"id"`
	Name string                     `json:"name"`
	Args map[string]json.RawMessage `json:"args"`
	Seed int64                      `json:"seed"`
}

// GeneratedCaseError reports a generated case hc could not build: its
// generator failed, or built an invalid input or another than the one its
// digest names, or the reference solution failed on it. It is an error of
// the problem, not a verdict on the solution.
type GeneratedCaseError struct {
	// Path names generator.py or the reference solution, as shown to the
	// user.
	Path string
	// Case is the case that failed, or 0 when generator.py failed to load.
	Case   int
	Reason string
	// Detail is the traceback or the program's output.
	Detail string
}

func (e *GeneratedCaseError) Error() string {
	where := "cannot be loaded"
	if e.Case > 0 {
		where = fmt.Sprintf("failed on generated case %d", e.Case)
	}
	msg := fmt.Sprintf("%s %s", e.Path, where)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	msg += " (the problem's generated cases are broken; this is not a verdict on your solution)"
	if d := strings.TrimSpace(e.Detail); d != "" {
		msg += "\n" + d
	}
	return msg
}

// GenerateOptions configure Generate.
type GenerateOptions struct {
	Generator lang.Generator
	// Timeout limits the generator process.
	Timeout time.Duration
	// Name is how errors refer to generator.py, e.g. relative to the
	// workspace; its absolute path if empty.
	Name string
	// CheckDigest requires every input to match its case's digest, when
	// the case has one.
	CheckDigest bool
}

// Generate builds the inputs of the generated cases among cases with the
// problem's generator.py, in one process, and sets them. Cases that are not
// generated, or already have an input, are left alone. A generator that
// fails, or an input that is invalid or does not match its digest, is a
// *GeneratedCaseError.
func Generate(ctx context.Context, p *problem.Problem, cases []Selected, o GenerateOptions) error {
	var items []GenerateItem
	for _, s := range cases {
		if g := s.Case.Generate; g != nil && s.Case.Input == nil {
			items = append(items, GenerateItem{ID: s.Index, Name: g.Name, Args: g.Args, Seed: g.Seed})
		}
	}
	if len(items) == 0 {
		return nil
	}
	path, name := p.GeneratorPath(), o.Name
	if name == "" {
		name = path
	}
	fail := func(id int, reason, detail string) error {
		return &GeneratedCaseError{Path: name, Case: id, Reason: reason, Detail: strings.ReplaceAll(detail, path, name)}
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("case %d is generated, but %s is missing", items[0].ID, name)
	}
	if o.Generator == nil {
		return errors.New("no generator runner configured")
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Second
	}
	for i := range items {
		if items[i].Args == nil {
			items[i].Args = map[string]json.RawMessage{}
		}
	}
	work, err := os.MkdirTemp("", "hc-gen-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	itemsPath, out := filepath.Join(work, "items.json"), filepath.Join(work, "inputs.jsonl")
	if err := os.WriteFile(itemsPath, data, 0o644); err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	cmd, err := o.Generator.GenerateCommand(runCtx, work, path, itemsPath, out)
	if err != nil {
		return err
	}
	output := &tailBuffer{max: 16 << 10}
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = output, output
	cmd.WaitDelay = time.Second
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	detail := output.String()
	if runErr != nil && !timedOut {
		detail = strings.TrimSpace(strings.TrimSpace(detail) + "\n" + runErr.Error())
	}
	results, err := readGenerated(out)
	if err != nil {
		return err
	}
	if r, ok := results[0]; ok {
		return fail(0, "", r.Error)
	}
	for i := range cases {
		c := &cases[i].Case
		if c.Generate == nil || c.Input != nil {
			continue
		}
		id := cases[i].Index
		r, ok := results[id]
		switch {
		case ok && r.Error != "":
			return fail(id, "", r.Error)
		case !ok && timedOut:
			return fail(id, fmt.Sprintf("time limit of %s exceeded", o.Timeout), detail)
		case !ok:
			return fail(id, "exited without a result", detail)
		}
		var input map[string]json.RawMessage
		if err := json.Unmarshal(r.Input, &input); err != nil || input == nil {
			return fail(id, "the input is not a JSON object", "")
		}
		c.Input = input
		if err := p.ValidateCase(*c); err != nil {
			c.Input = nil
			return fail(id, fmt.Sprintf("%s built an invalid input: %v", c.Generate, err), "")
		}
		if o.CheckDigest && c.Digest != "" {
			d, err := problem.InputDigest(input)
			if err != nil {
				return err
			}
			if d != c.Digest {
				c.Input = nil
				return fail(id, fmt.Sprintf("%s built another input than the author's (digest %s…, not %s…): generators must be deterministic, and a changed one needs hc cases build", c.Generate, d[:19], c.Digest[:19]), "")
			}
		}
	}
	return nil
}

type generated struct {
	ID    int             `json:"id"`
	Input json.RawMessage `json:"input"`
	Error string          `json:"error"`
}

func readGenerated(path string) (map[int]generated, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[int]generated{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	results := map[int]generated{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<30)
	for sc.Scan() {
		var r generated
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			// A partially written last line after a crash; the case is
			// then reported as left without a result.
			continue
		}
		results[r.ID] = r
	}
	return results, sc.Err()
}
