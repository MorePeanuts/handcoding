package javascript_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang/javascript"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestTypeScriptRelativeImports(t *testing.T) {
	t.Parallel()
	testenv.LookPath(t, "node")
	testenv.LookPath(t, "npm")
	testenv.LookPath(t, "tsc")
	l := javascript.Language{TypeScript: true, Environment: t.TempDir()}
	for _, mode := range []string{"commonjs", "module"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			ext, specifier, emitted := ".ts", "", ".js"
			if mode == "module" {
				ext, specifier, emitted = ".cts", ".cjs", ".cjs"
			}
			p := &problem.Problem{Dir: dir, Kind: problem.KindFunction, Function: &problem.Function{
				Name: "f", Params: []problem.Param{{Name: "n", Type: "integer"}}, Returns: "integer",
			}, Judge: problem.Judge{Compare: problem.CompareExact}}
			source := fmt.Sprintf(`import {twice} from './helpers/helper%s';
const config = require('./config.json');
function f(n:number):number { return twice(n) + config.offset; }
`, specifier)
			files := map[string]string{
				"solution.ts":           source,
				"reference/solution.ts": source,
				"package.json":          fmt.Sprintf(`{"type":%q}`, mode),
				"config.json":           `{"offset":1}`,
				"helpers/helper" + ext: fmt.Sprintf(`import {sum} from './nested%s';
import {Queue} from '@datastructures-js/queue';
import {basename} from 'node:path';
export function twice(n:number):number {
    if (n < 0) throw new Error('helper failure');
    if (basename('/a/b') !== 'b') throw new Error('builtin import failed');
    return sum(new Queue<number>([n]).dequeue(), n);
}
`, specifier),
				"helpers/nested" + ext: `export function sum(a:number,b:number):number { return a+b; }`,
				// A stale source-side build must never shadow the fresh emitted helper.
				"helpers/helper" + emitted: `module.exports.twice = () => 9999;`,
			}
			for name, content := range files {
				file := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cases, err := problem.ParseCases(strings.NewReader(`
{"input":{"n":7},"expected":15}
{"input":{"n":-1},"expected":0}
{"input":{"n":2},"expected":5}
`), "cases")
			if err != nil {
				t.Fatal(err)
			}
			sel := make([]runner.Selected, len(cases))
			for i, c := range cases {
				sel[i] = runner.Selected{Index: i + 1, Case: c}
			}
			rep, err := runner.Run(context.Background(), l, p, dir, sel, runner.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if rep.CompileError != "" || len(rep.Cases) != 3 {
				t.Fatalf("%+v", rep)
			}
			if rep.Cases[0].Verdict != runner.Accepted || rep.Cases[1].Verdict != runner.RuntimeError || rep.Cases[2].Verdict != runner.Accepted {
				t.Fatalf("cases=%+v stderr=%s", rep.Cases, rep.Stderr)
			}
			if !strings.Contains(rep.Cases[1].Error, "helper failure") || !strings.Contains(rep.Cases[1].Error, filepath.Join(dir, "helpers/helper"+ext)+":") {
				t.Fatalf("helper stack did not map to original source: %s", rep.Cases[1].Error)
			}
		})
	}
}
