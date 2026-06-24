package model

import "time"

// FileEntry is one row from an `adb shell ls -la` listing.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Mode    string    `json:"mode"` // permission bits, e.g. "drwxr-xr-x"
	Link    string    `json:"link"` // symlink target, empty if not a link
}
