package cpp

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

// runtimeObject shares the problem-independent worker across solution builds.
// Its key covers preprocessed source (including system headers), the compiler,
// flags and environment. Problem-specific codecs and calls stay in the solution
// translation unit; neither solution code nor judge Node variants enter here.
func (l Language) runtimeObject(ctx context.Context, dir string, flags []string) (string, error) {
	env, err := filepath.Abs(l.environment(dir, jsonDependency))
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
	sourceDir, err := runtimeSources(env)
	if err != nil {
		return "", err
	}
	source := filepath.Join(sourceDir, "hc_core.cpp")
	flags = append(append([]string{}, flags...), "-I", filepath.Join(env, "include"))
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
	stage, err := os.CreateTemp(objects, ".runtime-*.o")
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

// Publish immutable sources atomically: concurrent hc processes may be building
// different problems, or different hc versions, in the same environment.
func runtimeSources(env string) (string, error) {
	h := sha256.New()
	h.Write(coreHeader)
	h.Write(coreSource)
	root := filepath.Join(env, "runtime")
	target := filepath.Join(root, fmt.Sprintf("%x", h.Sum(nil)))
	if _, err := os.Stat(filepath.Join(target, "hc_core.cpp")); err == nil {
		return target, nil
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(root, ".runtime-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	for name, data := range map[string][]byte{"hc_core.hpp": coreHeader, "hc_core.cpp": coreSource} {
		if err := os.WriteFile(filepath.Join(stage, name), data, 0644); err != nil {
			return "", err
		}
	}
	if err := os.Rename(stage, target); err != nil {
		if _, e := os.Stat(filepath.Join(target, "hc_core.cpp")); e != nil {
			return "", err
		}
	}
	return target, nil
}
