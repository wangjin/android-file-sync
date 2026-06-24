package main

import (
	"androidfs/internal/localfs"
	"androidfs/internal/model"
)

// ListLocalDir lists a directory on the host machine (the local pane).
// Returns the same FileEntry shape as the device pane so both render alike.
func (a *App) ListLocalDir(dir string) ([]model.FileEntry, error) {
	return localfs.ListDir(dir)
}

// DeleteLocal removes a file or directory (recursively) on the host.
func (a *App) DeleteLocal(path string) error {
	return localfs.Remove(path)
}
