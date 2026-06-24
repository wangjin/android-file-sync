package adb

import (
	"context"
	"runtime"
	"testing"
)

func TestPushEmitsProgress(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not run on windows CI")
	}
	// Fake adb prints progress lines to stderr then exits 0.
	body := `printf '1204/4500 (27%%)\n4500/4500 (100%%)\n' 1>&2
`
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)

	var seen []struct {
		bytes int64
		total int64
	}
	err := c.Push(context.Background(), "dev1", "/tmp/a", "/sdcard/a",
		func(b, tot int64) {
			seen = append(seen, struct {
				bytes int64
				total int64
			}{b, tot})
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[1].bytes != 4500 {
		t.Fatalf("progress = %+v", seen)
	}
}

func TestPushCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not run on windows CI")
	}
	body := `sleep 5
`
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Push(ctx, "dev1", "/tmp/a", "/sdcard/a", nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}
