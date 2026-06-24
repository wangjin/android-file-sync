package adb

import (
	"context"

	"androidfs/internal/model"
)

// ListDevices returns currently attached devices parsed from `adb devices -l`.
func (c *AdbClient) ListDevices(ctx context.Context) ([]model.Device, error) {
	out, _, err := c.run(ctx, "devices", "-l")
	if err != nil {
		return nil, err
	}
	return ParseDevices(out), nil
}

// Connect attaches a wireless device: `adb connect ip:port`.
func (c *AdbClient) Connect(ctx context.Context, addr string) (string, error) {
	out, _, err := c.run(ctx, "connect", addr)
	return out, err
}
