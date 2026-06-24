package adb

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// bytesContainer is a minimal strings.Builder implementing io.Writer.
type bytesContainer struct {
	b strings.Builder
}

func (c *bytesContainer) Write(p []byte) (int, error) { return c.b.Write(p) }
func (c *bytesContainer) String() string              { return c.b.String() }

// ioPipe returns a connected reader/writer pair (io.Pipe wrapper).
func ioPipe() (io.ReadCloser, io.WriteCloser) {
	return io.Pipe()
}

func scanLines(r io.Reader, onLine func(string)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		onLine(sc.Text())
	}
}

// Shell runs a shell command on the device and returns its stdout.
func (c *AdbClient) Shell(ctx context.Context, serial, command string) (string, error) {
	out, _, err := c.run(ctx, "-s", serial, "shell", command)
	return out, err
}
