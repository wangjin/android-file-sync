package adb

import (
	"context"
	"os/exec"
)

// AdbClient wraps a specific adb binary path. Tests inject a fake binary;
// production passes the extracted platform-tools path.
type AdbClient struct {
	bin string
}

func NewClient(binPath string) *AdbClient {
	return &AdbClient{bin: binPath}
}

// run executes adb with the given args and returns stdout, stderr, error.
func (c *AdbClient) run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	hideWindow(cmd)
	var stdout, stderr bytesContainer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runStream executes adb and streams stderr line-by-line to onLine (used for
// push/pull progress). stdout is collected and returned.
func (c *AdbClient) runStream(ctx context.Context, onLine func(string), args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	hideWindow(cmd)
	pr, pw := ioPipe()
	cmd.Stderr = pw
	var stdout bytesContainer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan struct{})
	go func() {
		scanLines(pr, onLine)
		close(done)
	}()
	err := cmd.Wait()
	pw.Close()
	<-done
	return stdout.String(), err
}

// RunVersion runs `adb version` as a startup health check.
func (c *AdbClient) RunVersion(ctx context.Context) (string, string, error) {
	return c.run(ctx, "version")
}
