package adb

import (
	"context"
	"fmt"
	"strings"

	"androidfs/internal/model"
)

// ListDir lists one directory on the device using `adb shell ls -la`.
func (c *AdbClient) ListDir(ctx context.Context, serial, dir string) ([]model.FileEntry, error) {
	out, err := c.Shell(ctx, serial, "ls -la "+quoteArg(dir))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	return ParseListing(dir, out)
}

// Stat returns a single entry, or nil if not found.
func (c *AdbClient) Stat(ctx context.Context, serial, p string) (*model.FileEntry, error) {
	entries, err := c.ListDir(ctx, serial, parentDir(p))
	if err != nil {
		return nil, err
	}
	base := p[strings.LastIndex(p, "/")+1:]
	for i := range entries {
		if entries[i].Name == base {
			return &entries[i], nil
		}
	}
	return nil, nil
}

func parentDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return "/"
	}
	return p[:i]
}

// quoteArg wraps a path in single quotes for the device shell so spaces and
// most special characters are safe. Single quotes inside the path are escaped.
func quoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
