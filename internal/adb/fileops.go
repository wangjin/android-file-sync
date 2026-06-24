package adb

import "context"

// Mkdir creates a directory (and parents) on the device.
func (c *AdbClient) Mkdir(ctx context.Context, serial, p string) error {
	_, err := c.Shell(ctx, serial, "mkdir -p "+quoteArg(p))
	return err
}

// Rename moves/renames a path on the device.
func (c *AdbClient) Rename(ctx context.Context, serial, oldP, newP string) error {
	_, err := c.Shell(ctx, serial, "mv "+quoteArg(oldP)+" "+quoteArg(newP))
	return err
}

// Delete removes a file or directory recursively on the device.
func (c *AdbClient) Delete(ctx context.Context, serial, p string) error {
	_, err := c.Shell(ctx, serial, "rm -rf "+quoteArg(p))
	return err
}
