package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// SyncSettings upgrades settings on ordinary workspace opens, including
// development builds and authoring workspaces. Read-only commands do not call it.
func (a *App) SyncSettings(ctx context.Context) error {
	var locales map[string]string
	// Do not create a database merely to check for preferences from older hc.
	if _, err := os.Stat(filepath.Join(a.WS.StatePath(), "hc.db")); err == nil {
		st, err := a.Store()
		if err != nil {
			return err
		}
		locales, err = st.Prefs(ctx, localePref)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := a.WS.SyncSettings(locales); err != nil {
		return err
	}
	// The YAML is durable before old values are removed; retrying is safe.
	for name := range locales {
		if err := a.store.SetPref(ctx, name, localePref, ""); err != nil {
			return err
		}
	}
	a.Langs = Languages(a.WS)
	return nil
}
