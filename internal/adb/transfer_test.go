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
	// Fake adb prints a single summary line to stderr then exits 0 — that is
	// exactly what real adb 37 does (no per-chunk streaming).
	body := `printf '/tmp/a: 1 file pushed, 0 skipped. 14.9 MB/s (4500 bytes in 0.001s)\n' 1>&2
`
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)

	type sample struct {
		bytes, total, rate int64
	}
	var seen []sample
	err := c.Push(context.Background(), "dev1", "/tmp/a", "/sdcard/a",
		func(b, tot, rate int64) {
			seen = append(seen, sample{b, tot, rate})
		})
	if err != nil {
		t.Fatal(err)
	}
	// adb prints exactly one summary line; that yields one progress callback.
	if len(seen) != 1 {
		t.Fatalf("progress callbacks = %d want 1: %+v", len(seen), seen)
	}
	if seen[0].bytes != 4500 || seen[0].total != 4500 {
		t.Fatalf("got bytes=%d total=%d want 4500/4500", seen[0].bytes, seen[0].total)
	}
	// 14.9 MB/s parsed from the token.
	if seen[0].rate < 14000000 || seen[0].rate > 16000000 {
		t.Fatalf("rate=%d want ~14.9MB/s", seen[0].rate)
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
