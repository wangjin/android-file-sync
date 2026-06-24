package adb

import "context"

// ProgressFn receives (bytesTransferred, totalBytes) as adb reports them.
type ProgressFn func(bytes, total int64)

// Push uploads local -> remote on the device, calling onProgress for each
// stderr progress line. Respects ctx cancellation.
func (c *AdbClient) Push(ctx context.Context, serial, local, remote string, onProgress ProgressFn) error {
	_, err := c.runStream(ctx, func(line string) {
		b, tot, ok := ParseProgress(line)
		if ok && onProgress != nil {
			onProgress(b, tot)
		}
	}, "-s", serial, "push", local, remote)
	return err
}

// Pull downloads remote -> local on the host.
func (c *AdbClient) Pull(ctx context.Context, serial, remote, local string, onProgress ProgressFn) error {
	_, err := c.runStream(ctx, func(line string) {
		b, tot, ok := ParseProgress(line)
		if ok && onProgress != nil {
			onProgress(b, tot)
		}
	}, "-s", serial, "pull", remote, local)
	return err
}
