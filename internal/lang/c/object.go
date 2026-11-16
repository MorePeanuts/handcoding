package c

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

// dependencyObject keeps yyjson's expensive optimized compilation out of every
// solution build. Preprocessing is cheap and includes system/header changes in
// the key, in addition to the compiler, flags and environment. Never cache the
// user's translation unit here.
func (l Language) dependencyObject(ctx context.Context, dir string, flags []string) (string, error) {
	env, err := filepath.Abs(l.environment(dir))
	if err != nil {
		return "", err
	}
	compiler, err := lang.ResolveExecutable(l.compiler())
	if err != nil {
		return "", err
	}
	info, err := os.Stat(compiler)
	if err != nil {
		return "", err
	}
	version, err := lang.Probe(ctx, compiler, nil, "--version")
	if err != nil {
		return "", err
	}
	source := filepath.Join(env, "yyjson.c")
	args := append(append([]string{}, flags...), "-E", source)
	cmd := exec.CommandContext(ctx, compiler, args...)
	cmd.Dir = dir
	preprocessed, err := cmd.Output()
	if err != nil {
		if e, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", &lang.CompileError{Output: string(e.Stderr) + "\n" + err.Error()}
		}
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%q\n%q\n%d:%d\n%q\n%q\n", compiler, version, info.Size(), info.ModTime().UnixNano(), flags, os.Environ())
	h.Write(preprocessed)
	objects := filepath.Join(env, "objects")
	if err := os.MkdirAll(objects, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(objects, fmt.Sprintf("%x.o", h.Sum(nil)))
	unlock, err := lockFile(ctx, target+".lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	if st, err := os.Stat(target); err == nil && st.Mode().IsRegular() && st.Size() > 0 {
		return target, nil
	}
	stage, err := os.CreateTemp(objects, ".yyjson-*.o")
	if err != nil {
		return "", err
	}
	stage.Close()
	defer os.Remove(stage.Name())
	args = append(append([]string{}, flags...), "-c", source, "-o", stage.Name())
	cmd = exec.CommandContext(ctx, compiler, args...)
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", &lang.CompileError{Output: string(b) + "\n" + err.Error()}
	}
	if err := os.Rename(stage.Name(), target); err != nil {
		return "", err
	}
	return target, nil
}
