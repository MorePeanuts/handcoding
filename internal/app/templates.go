package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

// MigrateTemplates backfills native templates in a writable problem. Each
// problem is staged in full before publication, so a failed write cannot leave
// its template and C binding out of sync. Preview and conflict paths never
// write problem content.
func (a *App) MigrateTemplates(p *problem.Problem, q *leetcode.Question, site leetcode.Site, languages []string, dryRun bool) (*leetcode.TemplateUpdate, error) {
	if err := a.WS.CheckWritable(p); err != nil {
		return nil, err
	}
	release, err := a.WS.LockTemplateMigration()
	if err != nil {
		return nil, err
	}
	defer release()
	before, err := templateTreeDigest(p.Dir)
	if err != nil {
		return nil, err
	}
	// Re-read under the lock: another migration may have completed while the
	// caller was fetching the remote question.
	current, err := problem.Load(p.Dir)
	if err != nil {
		return nil, err
	}
	u, err := leetcode.PlanTemplates(current, q, site, languages)
	if err != nil {
		return nil, err
	}
	if len(u.Conflicts) > 0 || dryRun || len(u.Files) == 0 {
		return u, nil
	}
	if err := publishTemplates(p.Dir, u.Files, before, os.Rename); err != nil {
		return u, err
	}
	return u, nil
}

func templateTreeDigest(dir string) ([]byte, error) {
	// Refuse indirection rather than modifying a template outside this
	// problem or silently changing symlink semantics in a copied directory.
	h := sha256.New()
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("cannot migrate non-regular file %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", rel, info.Mode())
		if entry.IsDir() {
			return nil
		}
		fmt.Fprintf(h, "%d\x00", info.Size())
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(h, f)
		return errors.Join(err, f.Close())
	}); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func publishTemplates(dir string, files map[string][]byte, before []byte, rename func(string, string) error) error {
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".hc-templates-")
	if err != nil {
		return err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(tmp)
		}
	}()
	next := filepath.Join(tmp, "new")
	if err := os.CopyFS(next, os.DirFS(dir)); err != nil {
		return err
	}
	for rel, data := range files {
		path := filepath.Join(next, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	if _, err := problem.Load(next); err != nil {
		return err
	}
	// CopyFS adds write permissions. Restore the modes of existing content.
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		return os.Chmod(filepath.Join(next, rel), info.Mode().Perm())
	}); err != nil {
		return err
	}
	// Editors and other commands do not take the migration lock. Detect
	// changes made while planning/copying instead of replacing their work.
	after, err := templateTreeDigest(dir)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return errors.New("problem changed during template migration; no changes applied, run the preview again")
	}
	// Nest both copies below the staging directory so pack discovery never
	// mistakes a backup for a second problem with the same identity.
	backup := filepath.Join(tmp, "original")
	if err := rename(dir, backup); err != nil {
		return err
	}
	if err := rename(next, dir); err != nil {
		if restore := rename(backup, dir); restore != nil {
			keepBackup = true
			return errors.Join(err, fmt.Errorf("restore failed; original problem is at %s: %w", backup, restore))
		}
		return err
	}
	return os.RemoveAll(backup)
}
