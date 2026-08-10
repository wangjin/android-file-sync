package main

import "androidfs/internal/model"

// ListDir lists a directory on the given device.
func (a *App) ListDir(serial, path string) ([]model.FileEntry, error) {
	return a.client.ListDir(a.ctx, serial, path)
}

// ReadImage pulls a device image's raw bytes for in-app preview (no local
// download). Returns (bytes, mime, error). Files over the size cap are
// rejected. See adb.ReadImage for details.
func (a *App) ReadImage(serial, path string) ([]byte, string, error) {
	return a.client.ReadImage(a.ctx, serial, path)
}

// HomePath returns the host user home directory (default for local pane).
func (a *App) HomePath() (string, error) {
	return hostHome()
}
