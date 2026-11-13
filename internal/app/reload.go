package app

import "github.com/MorePeanuts/handcoding/internal/workspace"

// ReloadWorkspace reopens configuration and language environments after an
// external command. Call only while no operation is running. The App retains
// its identity and ownership, including the caller's deferred Close.
func (a *App) ReloadWorkspace() error {
	ws, err := workspace.Open(a.WS.Root)
	if err != nil {
		return err
	}
	if a.store != nil {
		err = a.store.Close()
		a.store = nil
		if err != nil {
			return err
		}
	}
	a.WS = ws
	a.Langs = Languages(ws)
	return nil
}
