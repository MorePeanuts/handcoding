package c_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/runner"
)

func TestExplicitBindingsAndRoundTrip(t *testing.T) {
	t.Parallel()
	l := language(t)
	for _, name := range []string{"bound-function", "bound-counter"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, e := problem.Load(filepath.Join("testdata", name))
			if e != nil {
				t.Fatal(e)
			}
			p.Judge.Compare = problem.CompareExact
			p.C.Libraries = nil // GLib requirements are tested separately.
			src := `int actualIdentity(int x){return x;}`
			cs := `{"input":{"value":17},"expected":17}`
			if name == "bound-counter" {
				src = `struct State{int value;}; struct State*makeState(int x){struct State*s=malloc(sizeof(*s));s->value=x;return s;} int stepState(struct State*s){return ++s->value;} void destroyState(struct State*s){free(s);}`
				cs = `{"input":{"ops":["Counter","next","next"],"args":[[1],[],[]]},"expected":[null,2,3]}`
			}
			rep := run(t, l, p, src, cs, runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("%+v", rep)
			}
		})
	}
	for _, instances := range []string{problem.InstancesSame, problem.InstancesSeparate} {
		t.Run(instances, func(t *testing.T) {
			t.Parallel()
			p := &problem.Problem{Kind: problem.KindClass, Class: &problem.Class{Name: "Codec", Methods: []problem.Method{{Name: "serialize", Params: []problem.Param{{Name: "root", Type: "TreeNode"}}, Returns: "string"}, {Name: "deserialize", Params: []problem.Param{{Name: "data", Type: "string"}}, Returns: "TreeNode"}}}, Judge: problem.Judge{Compare: problem.CompareExact, RoundTrip: &problem.RoundTrip{Encode: "serialize", Decode: "deserialize", Instances: instances}}}
			want := "=="
			if instances == problem.InstancesSeparate {
				want = "!="
			}
			src := `typedef struct{int id;} Codec; static int ids; Codec*codecCreate(void){Codec*c=malloc(sizeof(*c));c->id=++ids;return c;} char*codecSerialize(Codec*c,struct TreeNode*r){char*s=malloc(32);sprintf(s,"%d",c->id);return s;} struct TreeNode*codecDeserialize(Codec*c,char*s){struct TreeNode*r=calloc(1,sizeof(*r));r->val=(c->id ` + want + ` atoi(s));return r;} void codecFree(Codec*c){free(c);}`
			rep := run(t, l, p, src, `{"input":{"root":[1]},"expected":[1]}`, runner.Options{})
			if rep.Verdict != runner.Accepted {
				t.Fatalf("%+v", rep)
			}
		})
	}
}

func TestTerminalDebugBinding(t *testing.T) {
	t.Parallel()
	l := language(t)
	dir := t.TempDir()
	p := function("integer", "integer")
	p.C = &problem.CBinding{Function: "nativeEntry"}
	if e := os.WriteFile(filepath.Join(dir, "solution.c"), []byte("int nativeEntry(int x){return x;}\n"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := l.Prepare(dir, p); e != nil {
		t.Fatal(e)
	}
	if _, e := l.DebugCommand(dir, dir, p, lang.DebugOptions{Headless: true}); e == nil {
		t.Fatal("headless accepted")
	}
	for _, debugger := range []string{"GDB", "LLDB"} {
		t.Run(debugger, func(t *testing.T) {
			l := l
			l.Debugger = filepath.Join(t.TempDir(), "wrapper")
			if e := os.WriteFile(l.Debugger, []byte("#!/bin/sh\necho '"+debugger+" 18.1.0'\n"), 0755); e != nil {
				t.Fatal(e)
			}
			cmd, e := l.DebugCommand(dir, dir, p, lang.DebugOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(strings.Join(cmd.Args, " "), "nativeEntry") || cmd.Dir != dir {
				t.Fatal(cmd)
			}
		})
	}
}

func TestLocalGLib(t *testing.T) {
	t.Parallel()
	l := language(t)
	l.GLib = "required"
	if e := exec.Command("pkg-config", "--exists", "glib-2.0").Run(); e != nil {
		if os.Getenv("HC_TEST_REQUIRE_GLIB") == "1" {
			t.Fatal(e)
		}
		checks := l.Diagnose(context.Background(), lang.DiagnosticOptions{})
		for _, d := range checks {
			if d.Role == "GLib" && d.Status == "missing" {
				t.Log("GLib not installed; verified missing dependency diagnostic")
				return
			}
		}
		t.Fatal(checks)
	}
	p := function("integer", "integer")
	p.C = &problem.CBinding{Libraries: []string{"glib"}}
	rep := run(t, l, p, `int f(int x){GHashTable*t=g_hash_table_new(g_direct_hash,g_direct_equal);g_hash_table_insert(t,GINT_TO_POINTER(1),GINT_TO_POINTER(x));int y=GPOINTER_TO_INT(g_hash_table_lookup(t,GINT_TO_POINTER(1)));g_hash_table_destroy(t);return y;}`, `{"input":{"x":42},"expected":42}`, runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
}

func TestUthashAndAddressSanitizer(t *testing.T) {
	t.Parallel()
	l := language(t)
	l.Sanitizers = "address"
	src := `struct Entry{int key,value;UT_hash_handle hh;};int f(int x){struct Entry*head=NULL,*e=calloc(1,sizeof(*e));e->key=1;e->value=x;HASH_ADD_INT(head,key,e);int key=1;struct Entry*found;HASH_FIND_INT(head,&key,found);int y=found->value;HASH_DEL(head,e);free(e);return y;}`
	rep := run(t, l, function("integer", "integer"), src, `{"input":{"x":42},"expected":42}`, runner.Options{})
	if rep.Verdict != runner.Accepted {
		t.Fatalf("%+v", rep)
	}
	rep = run(t, l, function("integer", "integer"), `int f(int x){int*p=malloc(sizeof(int));volatile int*q=p;free(p);return q[x];}`, `{"input":{"x":0},"expected":0}`, runner.Options{})
	if rep.Verdict != runner.RuntimeError || !strings.Contains(rep.Stderr, "AddressSanitizer") {
		t.Fatalf("%+v", rep)
	}
}
