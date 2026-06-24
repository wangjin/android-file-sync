package main

import "androidfs/internal/model"

// ListDir lists a directory on the given device.
func (a *App) ListDir(serial, path string) ([]model.FileEntry, error) {
	return a.client.ListDir(a.ctx, serial, path)
}

// HomePath returns the host user home directory (default for local pane).
func (a *App) HomePath() (string, error) {
	return hostHome()
}
