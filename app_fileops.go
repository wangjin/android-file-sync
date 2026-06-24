package main

// Mkdir creates a directory on the device.
func (a *App) Mkdir(serial, path string) error {
	return a.client.Mkdir(a.ctx, serial, path)
}

// Rename renames a path on the device.
func (a *App) Rename(serial, oldPath, newPath string) error {
	return a.client.Rename(a.ctx, serial, oldPath, newPath)
}

// Delete removes a file/dir on the device.
func (a *App) Delete(serial, path string) error {
	return a.client.Delete(a.ctx, serial, path)
}
