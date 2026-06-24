package adb

import (
	"errors"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"androidfs/internal/model"
)

// listingRe matches a single `ls -la` row.
//
//	mode links owner group size date time name [-> target]
var listingRe = regexp.MustCompile(
	`^([bcdlsp-][rwxsStT-]{9})\s+\d+\s+\S+\s+\S+\s+(\d+)\s+(\d{4}-\d{2}-\d{2})\s+(\d{2}:\d{2})\s+(.+)$`,
)

// ParseListing turns `adb shell ls -la` stdout into FileEntry rows for the
// given directory path. A "permission denied" message yields an error.
func ParseListing(dir, output string) ([]model.FileEntry, error) {
	if strings.Contains(output, "permission denied") {
		return nil, errors.New("permission denied")
	}
	var entries []model.FileEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		m := listingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mode := m[1]
		size, _ := strconv.ParseInt(m[2], 10, 64)
		modTime, _ := time.Parse("2006-01-02 15:04", m[3]+" "+m[4])
		nameField := m[5]

		link := ""
		if strings.HasPrefix(mode, "l") {
			if idx := strings.Index(nameField, " -> "); idx >= 0 {
				link = nameField[idx+4:]
				nameField = nameField[:idx]
			}
		}
		entry := model.FileEntry{
			Name:    nameField,
			Path:    path.Join(dir, nameField),
			IsDir:   mode[0] == 'd',
			Size:    size,
			ModTime: modTime,
			Mode:    mode,
			Link:    link,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
