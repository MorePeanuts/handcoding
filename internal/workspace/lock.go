package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// The locks commands hold, in StateDir.
const (
	// packLockFile is held by pack commands.
	packLockFile = "packs.lock"
	// skillsLockFile is held by commands writing hc's skills and its block
	// of AGENTS.md.
	skillsLockFile = "skills.lock"
)

// ErrPacksLocked is returned by LockPacks while another command holds the
// lock.
var ErrPacksLocked = errors.New("another hc pack command is running in this workspace; try again when it is done")

// LockPacks takes the lock that commands changing the installed packs hold
// while they change packs/ and the packs list of hc.yaml, so that two of
// them do not undo each other's work. It does not wait: while another
// command holds it, it fails with ErrPacksLocked. Once it has the lock, it
// reads the packs list of hc.yaml again, which another command may have
// changed since the workspace was opened. The operating system releases the
// lock when the process ends, so a command that crashes leaves none behind.
func (w *Workspace) LockPacks() (unlock func(), err error) {
	release, held, err := w.lock(packLockFile, false)
	if err != nil {
		return nil, err
	}
	if !held {
		return nil, ErrPacksLocked
	}
	if err := w.reloadPacks(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// LockSkills takes the lock that commands hold while they write the skills
// hc installs and its block of AGENTS.md, so that two hc processes never
// write them at once. With wait, it waits for another command to release
// it; without, it reports at once that it did not get it (held false). Like
// LockPacks's, the lock goes with the process.
func (w *Workspace) LockSkills(wait bool) (unlock func(), held bool, err error) {
	return w.lock(skillsLockFile, wait)
}

// lock takes the lock file name in StateDir.
func (w *Workspace) lock(name string, wait bool) (unlock func(), held bool, err error) {
	if err := os.MkdirAll(w.StatePath(), 0o755); err != nil {
		return nil, false, err
	}
	path := filepath.Join(w.StatePath(), name)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, false, err
	}
	if wait {
		held, err = true, lockFile(f)
	} else {
		held, err = tryLock(f)
	}
	if err != nil || !held {
		f.Close()
		if err != nil {
			return nil, false, fmt.Errorf("locking %s: %w", w.Rel(path), err)
		}
		return nil, false, nil
	}
	return func() {
		_ = unlockFile(f)
		f.Close()
	}, true, nil
}

// reloadPacks reads the packs list of hc.yaml again.
func (w *Workspace) reloadPacks() error {
	data, err := os.ReadFile(filepath.Join(w.Root, ConfigFile))
	if err != nil {
		return err
	}
	var cfg struct {
		Packs []PackSource `yaml:"packs"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("%s: %w", ConfigFile, err)
	}
	w.Config.Packs = cfg.Packs
	return nil
}
