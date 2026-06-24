package localfs

import (
	"os"
	"path/filepath"
	"strings"

	"androidfs/internal/model"
)

// ListDir reads a host directory and returns its entries as FileEntry values
// (the same shape the device pane uses), so both panes render identically.
func ListDir(dir string) ([]model.FileEntry, error) {
	infos, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]model.FileEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, entryFromDirEntry(dir, info))
	}
	return entries, nil
}

func entryFromDirEntry(dir string, info os.DirEntry) model.FileEntry {
	name := info.Name()
	full := filepath.Join(dir, name)
	fi, err := info.Info()
	mode := ""
	size := int64(0)
	var modTime interface{ IsZero() bool } // unused, kept nil
	_ = modTime
	if err == nil {
		mode = fi.Mode().String()
		size = fi.Size()
	}
	e := model.FileEntry{
		Name:  name,
		Path:  full,
		IsDir: info.IsDir(),
		Size:  size,
		Mode:  mode,
	}
	if err == nil {
		e.ModTime = fi.ModTime()
	}
	if info.Type()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(full); err == nil {
			e.Link = target
		}
	}
	return e
}

// ParentDir returns the parent of a path, with "/" at the top.
func ParentDir(p string) string {
	parent := filepath.Dir(p)
	// filepath.Dir("/") == "/" already; filepath.Dir("a") == "."
	if parent == "." {
		return ""
	}
	if !strings.HasPrefix(parent, "/") && parent != "" {
		return parent
	}
	return parent
}

// Remove deletes a file or directory (recursively) on the host.
func Remove(p string) error {
	return os.RemoveAll(p)
}
