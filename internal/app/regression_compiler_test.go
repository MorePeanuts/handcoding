package app_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	clang "github.com/MorePeanuts/handcoding/internal/lang/c"
	"github.com/MorePeanuts/handcoding/internal/lang/cpp"
	"github.com/MorePeanuts/handcoding/internal/testenv"
)

// Corpus builds enforce the minimum standard and reject extensions in the
// same compiler invocation that produces the executable. Ordinary driver tests
// still cover the configured/default standards and CheckMinVersion itself.
func strictCompiler(t *testing.T, compiler string) string {
	t.Helper()
	real := testenv.LookPath(t, compiler)
	wrapper := filepath.Join(t.TempDir(), "compiler")
	quoted := "'" + strings.ReplaceAll(real, "'", "'\"'\"'") + "'"
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+quoted+" -pedantic-errors \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return wrapper
}

func TestRegressionCompilerRejectsExtensions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ compiler, standard, extension string }{
		{clang.DefaultCompiler(), clang.MinStandard, ".c"},
		{cpp.DefaultCompiler(), cpp.MinStandard, ".cpp"},
	} {
		t.Run(tt.standard, func(t *testing.T) {
			t.Parallel()
			compiler := strictCompiler(t, tt.compiler)
			dir := t.TempDir()
			file := filepath.Join(dir, "solution"+tt.extension)
			for _, extension := range []bool{false, true} {
				source := "int main(void) { return 0; }"
				if extension {
					source = "int main(void) { return ({ 0; }); }" // GNU statement expression
				}
				if err := os.WriteFile(file, []byte(source+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
				out, err := exec.CommandContext(t.Context(), compiler, "-std="+tt.standard, file, "-o", filepath.Join(dir, "prog")).CombinedOutput()
				if (err != nil) != extension {
					t.Fatalf("extension=%v: %v\n%s", extension, err, out)
				}
			}
		})
	}
}
