package main

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"androidfs/internal/adb"
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

// RenameLocal renames (or moves) a file or directory on the host.
func (a *App) RenameLocal(oldPath, newPath string) error {
	return localfs.Rename(oldPath, newPath)
}

// ReadLocalImage reads up to adb.MaxImageBytes of a host image file's raw
// bytes for in-app preview. Returns (bytes, mime, error). Files over the cap
// are rejected so the frontend can show a friendly message instead.
func (a *App) ReadLocalImage(path string) ([]byte, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("stat image: %w", err)
	}
	if fi.Size() > adb.MaxImageBytes {
		return nil, "", fmt.Errorf("图片过大，已超过 %dMB 预览上限", adb.MaxImageBytes/(1024*1024))
	}
	out, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read image: %w", err)
	}
	mt := mime.TypeByExtension(filepath.Ext(path))
	if mt == "" {
		mt = "application/octet-stream"
	}
	return out, mt, nil
}
