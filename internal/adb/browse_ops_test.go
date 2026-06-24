package adb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestListDirViaShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not run on windows CI")
	}
	// Fake adb matches on the device shell command. `Shell` calls
	// `adb -s <serial> shell ls -la '<dir>'`, so the args contain
	// "-s dev1 shell ls -la '/sdcard'". Match the substring robustly.
	// The listing intentionally includes dotfiles (.thumbnails dir, .hidden
	// file) to assert they are filtered from the user-facing list.
	body := `case "$*" in
  *"shell ls -la '/sdcard'"*)
    echo 'total 8'
    echo 'drwxrwx--x 2 root root 4096 2026-06-20 13:00 DCIM'
    echo '-rw-rw---- 1 root root  100 2026-06-20 13:01 a.txt'
    echo 'drwxrwx--x 2 root root 4096 2026-06-20 13:02 .thumbnails'
    echo '-rw-rw---- 1 root root   42 2026-06-20 13:03 .hidden'
    ;;
esac
`
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)
	entries, err := c.ListDir(context.Background(), "dev1", "/sdcard")
	if err != nil {
		t.Fatal(err)
	}
	// Dotfiles (.thumbnails, .hidden) must be dropped — only DCIM and a.txt.
	if len(entries) != 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		t.Fatalf("got %d entries %v, want 2 (dotfiles not filtered)", len(entries), names)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name, ".") {
			t.Errorf("dotfile leaked into listing: %q", e.Name)
		}
	}
}

func TestMkdirRenameDeleteArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script not run on windows CI")
	}
	// Fake adb records the shell command to a temp file.
	dir := t.TempDir()
	rec := filepath.Join(dir, "cmd.txt")
	body := `echo "$*" >> ` + rec + `
`
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)
	ctx := context.Background()
	_ = c.Mkdir(ctx, "dev1", "/sdcard/new")
	_ = c.Rename(ctx, "dev1", "/sdcard/a", "/sdcard/b")
	_ = c.Delete(ctx, "dev1", "/sdcard/b")

	data, _ := os.ReadFile(rec)
	got := string(data)
	for _, want := range []string{"mkdir", "mv", "rm"} {
		if !contains(got, want) {
			t.Errorf("missing command %q in %q", want, got)
		}
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
