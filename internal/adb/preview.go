package adb

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// MaxImageBytes caps how much we pull for an in-app preview to avoid stalling
// the UI on very large device photos. ~30MB covers typical phone photos while
// keeping the round-trip snappy.
const MaxImageBytes = 30 * 1024 * 1024

// imageExts is the set of filename extensions considered previewable images.
// Keep this in sync with the frontend's isImageFile in frontend/src/lib/image.ts.
var imageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
}

// IsImagePath reports whether a path looks like a previewable image by extension.
// Case-insensitive. Used by the frontend gating (and kept here as the single
// source of truth for the extension set).
func IsImagePath(p string) bool {
	_, ok := imageExts[strings.ToLower(path.Ext(p))]
	return ok
}

// imageMime returns the MIME type for an image path, or
// "application/octet-stream" if the extension is unknown.
func imageMime(p string) string {
	if mt, ok := imageExts[strings.ToLower(path.Ext(p))]; ok {
		return mt
	}
	return "application/octet-stream"
}

// ReadImage pulls up to MaxImageBytes of a device file's raw bytes for an
// in-app preview (no local download). It runs `adb exec-out cat <path>`, which
// is binary-safe unlike `adb shell`. The MIME type is inferred from the
// extension. A file exceeding MaxImageBytes yields an error so the frontend can
// show a friendly message instead of fetching a multi-megabyte blob.
func (c *AdbClient) ReadImage(ctx context.Context, serial, remote string) ([]byte, string, error) {
	size, err := c.RemoteSize(ctx, serial, remote)
	if err != nil {
		return nil, "", fmt.Errorf("stat image: %w", err)
	}
	if size > MaxImageBytes {
		return nil, "", fmt.Errorf("图片过大 (%s)，已超过 %dMB 预览上限", humanBytes(size), MaxImageBytes/(1024*1024))
	}
	out, err := c.ExecOut(ctx, serial, "cat "+quoteArg(remote))
	if err != nil {
		return nil, "", fmt.Errorf("read image: %w", err)
	}
	return out, imageMime(remote), nil
}

// humanBytes renders a byte count for an error message, e.g. "12.3 MB".
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
