package adb

import (
	"context"
	"os"
)

// ProgressFn reports (bytesTransferred, totalBytes, bytesPerSec) as adb reports
// them. Because modern adb only prints a final summary line, this fires once
// per transfer with bytes == total and the average rate — there is no
// per-chunk streaming.
type ProgressFn func(bytes, total, rate int64)

// pushTotal returns the local file's size to seed a real Total for the bar.
func pushTotal(local string) int64 {
	if fi, err := os.Stat(local); err == nil && !fi.IsDir() {
		return fi.Size()
	}
	return 0
}

// Push uploads local -> remote on the device. It seeds Total from the local
// file size up front (so the bar is determinate), then reports the final
// transferred bytes + average rate parsed from adb's summary line.
func (c *AdbClient) Push(ctx context.Context, serial, local, remote string, onProgress ProgressFn) error {
	if onProgress != nil {
		if tot := pushTotal(local); tot > 0 {
			onProgress(0, tot, 0)
		}
	}
	_, err := c.runStream(ctx, func(line string) {
		b, tot, rate, ok := ParseProgress(line)
		if ok && onProgress != nil {
			onProgress(b, tot, rate)
		}
	}, "-s", serial, "push", local, remote)
	return err
}

// Pull downloads remote -> local on the host. Total is seeded from a device
// `stat` so the bar is determinate; the final line reports bytes + rate.
func (c *AdbClient) Pull(ctx context.Context, serial, remote, local string, onProgress ProgressFn) error {
	if onProgress != nil {
		if tot, err := c.RemoteSize(ctx, serial, remote); err == nil && tot > 0 {
			onProgress(0, tot, 0)
		}
	}
	_, err := c.runStream(ctx, func(line string) {
		b, tot, rate, ok := ParseProgress(line)
		if ok && onProgress != nil {
			onProgress(b, tot, rate)
		}
	}, "-s", serial, "pull", remote, local)
	return err
}
