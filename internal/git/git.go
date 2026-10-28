// Package git runs the git executable on PATH for the few operations that
// installing and updating packs needs. hc has no git library.
package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotInstalled is returned when there is no git on PATH.
var ErrNotInstalled = errors.New("git is not installed (or not on PATH); hc runs git to install and update packs")

// run runs git with args in dir and returns its standard output, without
// the final newline. Its standard error goes to progress when it is not nil:
// a file such as a terminal is git's own standard error, so that git shows
// its progress there; anything else gets git's messages. Otherwise the error
// and fatal lines git wrote make up the error it fails with.
func run(ctx context.Context, dir string, progress io.Writer, args ...string) (string, error) {
	exe, err := exec.LookPath("git")
	if err != nil {
		return "", ErrNotInstalled
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	// Messages in English, so that hc's errors read the same everywhere, and
	// no editor or pager.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_EDITOR=true", "GIT_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	shown := false
	switch w := progress.(type) {
	case nil:
	case *os.File:
		cmd.Stderr, shown = w, true
	default:
		cmd.Stderr = io.MultiWriter(w, &stderr)
	}
	if err := cmd.Run(); err != nil {
		msg := messages(stderr.String())
		switch {
		case shown:
			msg = err.Error() + " (git said why above)"
		case msg == "":
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// messages are the lines of git's standard error that say what went wrong.
func messages(stderr string) string {
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(stderr))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "error:") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return strings.TrimSpace(stderr)
	}
	return strings.Join(lines, "\n")
}

// IsRepository reports whether dir is the top of a git work tree (it holds
// .git) or a bare repository.
func IsRepository(dir string) bool {
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
		return true
	}
	for _, name := range []string{"HEAD", "objects", "refs"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// Clone clones source, a URL or a path, into dir, which must be missing or
// empty. git's progress goes to progress.
func Clone(ctx context.Context, source, dir string, progress io.Writer) error {
	_, err := run(ctx, "", progress, "clone", "--", source, dir)
	return err
}

// Fetch fetches branches and tags from source, which becomes the URL of
// origin, into the clone in dir. Tags that moved are updated.
func Fetch(ctx context.Context, dir, source string, progress io.Writer) error {
	if _, err := run(ctx, dir, nil, "remote", "set-url", "origin", source); err != nil {
		return err
	}
	_, err := run(ctx, dir, progress, "fetch", "--prune", "--tags", "--force", "origin")
	return err
}

// Resolve returns the commit that ref names in the clone in dir, as of its
// last fetch: the default branch of origin when ref is empty, else the branch
// of origin called ref, the tag ref, or a commit.
func Resolve(ctx context.Context, dir, ref string) (string, error) {
	if ref == "" {
		if c, err := commit(ctx, dir, "refs/remotes/origin/HEAD"); err == nil {
			return c, nil
		}
		return "", errors.New("the source has no default branch: name a branch or tag with --ref")
	}
	if strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("invalid ref %q", ref)
	}
	for _, name := range []string{"refs/remotes/origin/" + ref, "refs/tags/" + ref, ref} {
		if c, err := commit(ctx, dir, name); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("the source has no branch or tag %q", ref)
}

// commit resolves name to a commit hash.
func commit(ctx context.Context, dir, name string) (string, error) {
	return run(ctx, dir, nil, "rev-parse", "--verify", "--quiet", name+"^{commit}")
}

// Head returns the commit checked out in dir.
func Head(ctx context.Context, dir string) (string, error) {
	return commit(ctx, dir, "HEAD")
}

// Checkout checks out commit in dir, detached from any branch. With force,
// local changes are discarded first, untracked files included.
func Checkout(ctx context.Context, dir, commit string, force bool) error {
	if force {
		if err := Discard(ctx, dir); err != nil {
			return err
		}
	}
	_, err := run(ctx, dir, nil, "checkout", "--quiet", "--detach", commit)
	return err
}

// Discard discards the local changes in dir: changed and untracked files.
func Discard(ctx context.Context, dir string) error {
	if _, err := run(ctx, dir, nil, "reset", "--quiet", "--hard"); err != nil {
		return err
	}
	_, err := run(ctx, dir, nil, "clean", "--quiet", "-ffd")
	return err
}

// Changes lists the files of the work tree in dir that differ from the
// commit checked out, untracked ones included, as git status --porcelain
// shows them.
func Changes(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, nil, "status", "--porcelain")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}
