package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"syscall"

	"github.com/MorePeanuts/handcoding/internal/pack"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// CleanOptions selects known, rebuildable caches. A scoped cleanup excludes
// shared programs, runtimes, tool caches and the LeetCode problem index.
type CleanOptions struct {
	DryRun  bool
	Pack    string
	Problem string
}

// CleanFile is a known cached file, relative to the workspace. Bytes is its
// logical file size, not a claim about filesystem blocks (tool caches may use
// hard links). Removed is false in a dry run or after a failed deletion.
type CleanFile struct {
	Path     string
	Category string
	Bytes    int64
	Removed  bool
}

// CleanResult reports the exact files inspected and successfully removed.
// Files may be populated even when Clean returns an error.
type CleanResult struct {
	Files []CleanFile
	// Directories are tool-owned cache directories, removed when empty.
	Directories []string
}

// Clean explicitly removes known cache files, preserving persistent state,
// unknown files and symlinks outside opaque tool-cache namespaces. A dry run takes only a shared, read-only directory
// lock and does not sync the workspace, set up a runtime or open the database.
func (a *App) Clean(o CleanOptions) (*CleanResult, error) {
	lock := a.WS.LockCacheCleanup
	if o.DryRun {
		lock = a.WS.UseCaches
	}
	release, err := lock()
	if err != nil {
		return nil, err
	}
	defer release()
	packName, slug, err := a.cleanScope(o)
	if err != nil {
		return nil, err
	}
	result := &CleanResult{}
	state := a.WS.StatePath()
	info, err := os.Lstat(state)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, fmt.Errorf("%s is not a directory; caches were not cleaned", a.WS.Rel(state))
	}
	workspaceRoot, err := os.OpenRoot(a.WS.Root)
	if err != nil {
		return result, err
	}
	defer workspaceRoot.Close()
	root, err := workspaceRoot.OpenRoot(".hc")
	if err != nil {
		return result, err
	}
	defer root.Close()
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if !cacheDirectory(name, packName != "") {
				return fs.SkipDir
			}
			parts := strings.Split(name, "/")
			if packName != "" && len(parts) >= 3 && parts[0] == "cache" && parts[2] != packName {
				return fs.SkipDir
			}
			if slug != "" && len(parts) >= 4 && parts[3] != slug {
				return fs.SkipDir
			}
			if toolCachePath(name) {
				result.Directories = append(result.Directories, path.Join(".hc", name))
			}
			return nil
		}
		ownedLink := entry.Type()&fs.ModeSymlink != 0 && (strings.HasPrefix(name, "cache/go-build/") || strings.HasPrefix(name, "cache/uv/"))
		if !entry.Type().IsRegular() && !ownedLink {
			return nil
		}
		category := cacheCategory(name, packName, slug)
		if category == "" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result.Files = append(result.Files, CleanFile{Path: path.Join(".hc", name), Category: category, Bytes: info.Size()})
		return nil
	})
	if err != nil || o.DryRun {
		return result, err
	}
	var failures []error
	for i := range result.Files {
		f := &result.Files[i]
		if err := root.Remove(strings.TrimPrefix(f.Path, ".hc/")); err != nil {
			failures = append(failures, fmt.Errorf("remove %s: %w", f.Path, err))
		} else {
			f.Removed = true
		}
	}
	for _, dir := range slices.Backward(result.Directories) {
		err := root.Remove(strings.TrimPrefix(dir, ".hc/"))
		// Nonempty directories contain a failed deletion or an unusual
		// file type that was not inventoried. Keep them and their contents.
		if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			failures = append(failures, fmt.Errorf("remove %s: %w", dir, err))
		}
	}
	return result, errors.Join(failures...)
}

func toolCachePath(name string) bool {
	return name == "cache/go-build" || strings.HasPrefix(name, "cache/go-build/") || name == "cache/uv" || strings.HasPrefix(name, "cache/uv/")
}

func (a *App) cleanScope(o CleanOptions) (packName, slug string, err error) {
	if o.Problem != "" {
		ref := o.Problem
		if o.Pack != "" {
			ref = o.Pack + "/" + ref
		}
		p, err := a.WS.Problem(ref)
		if err != nil {
			return "", "", err
		}
		return p.Pack, p.Slug, nil
	}
	if o.Pack != "" && o.Pack != pack.LocalName {
		// Also allow clearing orphan caches of a removed pack, using the
		// same name validation as pack management.
		if err := checkPackName(o.Pack); err != nil {
			return "", "", err
		}
	}
	return o.Pack, "", nil
}

func cacheDirectory(name string, scoped bool) bool {
	if name == "." || name == "cache" {
		return true
	}
	parts := strings.Split(name, "/")
	if len(parts) >= 2 && parts[0] == "cache" && slices.Contains([]string{"cases", "baseline", "lastrun"}, parts[1]) {
		return len(parts) <= 4
	}
	if scoped {
		return false
	}
	if name == "cache/build" || name == "cache/build/go" || name == "runtime" || name == "runtime/python" || name == "runtime/python/__pycache__" {
		return true
	}
	if len(parts) == 4 && strings.Join(parts[:3], "/") == "cache/build/go" {
		return lowerHex(parts[3], 32)
	}
	if len(parts) >= 3 && parts[0] == "runtime" && parts[1] == "python" && lowerHex(parts[2], 12) {
		return len(parts) == 3 || len(parts) == 4 && parts[3] == "__pycache__"
	}
	for _, prefix := range []string{"cache/go-build", "cache/uv"} {
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	return false
}

func cacheCategory(name, packName, slug string) string {
	parts := strings.Split(name, "/")
	if len(parts) == 5 && parts[0] == "cache" && slices.Contains([]string{"cases", "baseline", "lastrun"}, parts[1]) {
		if packName != "" && parts[2] != packName || slug != "" && parts[3] != slug {
			return ""
		}
		file := strings.TrimSuffix(parts[4], ".json")
		if parts[1] == "cases" {
			file = strings.TrimSuffix(file, ".user")
		}
		if strings.HasSuffix(parts[4], ".json") && slices.Contains(problem.Languages(problem.Protocol), file) {
			return parts[1]
		}
		return ""
	}
	if packName != "" {
		return ""
	}
	if name == "leetcode-com-problems.json" || name == "leetcode-cn-problems.json" {
		return "LeetCode index"
	}
	if strings.HasPrefix(name, "cache/go-build/") {
		return "Go tool cache"
	}
	if strings.HasPrefix(name, "cache/uv/") {
		return "uv tool cache"
	}
	if len(parts) == 5 && strings.Join(parts[:3], "/") == "cache/build/go" && lowerHex(parts[3], 32) && (parts[4] == "prog" || strings.HasPrefix(parts[4], "prog.tmp-")) {
		return "Go programs"
	}
	if name == "runtime/python/hc_runtime.py" {
		return "Python runtime"
	}
	if len(parts) == 4 && parts[0] == "runtime" && parts[1] == "python" && parts[2] == "__pycache__" && strings.HasPrefix(parts[3], "hc_runtime.cpython-") && strings.HasSuffix(parts[3], ".pyc") {
		return "Python runtime"
	}
	if len(parts) >= 4 && parts[0] == "runtime" && parts[1] == "python" && lowerHex(parts[2], 12) {
		if len(parts) == 4 && slices.Contains([]string{"hc_runtime.py", "hc_checker.py", "hc_generator.py"}, parts[3]) {
			return "Python runtime"
		}
		if len(parts) == 5 && parts[3] == "__pycache__" && strings.HasSuffix(parts[4], ".pyc") {
			for _, prefix := range []string{"hc_runtime.cpython-", "hc_checker.cpython-", "hc_generator.cpython-"} {
				if strings.HasPrefix(parts[4], prefix) {
					return "Python runtime"
				}
			}
		}
	}
	return ""
}

func lowerHex(s string, n int) bool {
	return len(s) == n && strings.Trim(s, "0123456789abcdef") == ""
}
