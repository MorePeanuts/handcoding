package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreFile is the .gitignore at a practice workspace's root, which hc init
// writes so that a workspace kept in git holds only the user's own files.
const IgnoreFile = ".gitignore"

// The markers of hc's block in IgnoreFile. hc rewrites only what is between
// them; the rest of the file is the user's.
const (
	ignoreBegin = "# hc:begin"
	ignoreEnd   = "# hc:end"
)

// GeneratedFiles are the files the language drivers write next to a
// solution before every run (lang.Language's Prepare), which a workspace
// kept in git leaves out. TestGitignoreCoversGenerated (internal/app) keeps
// the list equal to what the drivers write.
var GeneratedFiles = []string{"go.mod", "zz_hc_driver.go", "_hc_runner.py", "hc_runtime.py", "__pycache__/"}

// IgnoreBlock is hc's block of a practice workspace's IgnoreFile: hc's state
// directory, the installed packs and what hc generates next to solutions.
func IgnoreBlock() string {
	var b strings.Builder
	b.WriteString(ignoreBegin + " (hc rewrites this block; the rest of the file is yours)\n")
	b.WriteString("# hc's local state: history, caches, the Python environment\n/" + StateDir + "/\n")
	b.WriteString("# installed packs (hc pack install reinstalls them from hc.yaml)\n/" + PacksDir + "/*\n!/" + PacksDir + "/local/\n")
	b.WriteString("# what hc writes next to your solutions before every run\n")
	for _, name := range GeneratedFiles {
		b.WriteString("/" + SolutionsDir + "/**/" + name + "\n")
	}
	b.WriteString(ignoreEnd + "\n")
	return b.String()
}

// IgnoreStale reports whether the workspace's IgnoreFile has hc's block and
// it is not IgnoreBlock. A file without the block, written by hand or by an
// older hc, is the user's: hc leaves it alone.
func (w *Workspace) IgnoreStale() (bool, error) {
	data, err := os.ReadFile(filepath.Join(w.Root, IgnoreFile))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	updated, ok := withIgnoreBlock(string(data))
	return ok && updated != string(data), nil
}

// RefreshIgnore rewrites hc's block of the workspace's IgnoreFile when it is
// stale (see IgnoreStale), keeping the rest of the file, and reports whether
// it did. It never adds a block to a file that has none.
func (w *Workspace) RefreshIgnore() (bool, error) {
	file := filepath.Join(w.Root, IgnoreFile)
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	updated, ok := withIgnoreBlock(string(data))
	if !ok || updated == string(data) {
		return false, nil
	}
	return true, writeAtomic(file, []byte(updated))
}

// withIgnoreBlock returns content with hc's block replaced by IgnoreBlock,
// and false when content has no complete block.
func withIgnoreBlock(content string) (string, bool) {
	begin := lineIndex(content, ignoreBegin)
	if begin < 0 {
		return content, false
	}
	end := lineIndex(content[begin:], ignoreEnd)
	if end < 0 {
		return content, false
	}
	end += begin + len(ignoreEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return content[:begin] + IgnoreBlock() + content[end:], true
}

// lineIndex returns the index of the first line of s that starts with
// prefix, or -1.
func lineIndex(s, prefix string) int {
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], prefix) {
			return i
		}
		nl := strings.IndexByte(s[i:], '\n')
		if nl < 0 {
			break
		}
		i += nl + 1
	}
	return -1
}
