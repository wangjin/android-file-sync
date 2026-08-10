package adb

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestIsImagePath(t *testing.T) {
	cases := map[string]bool{
		"photo.jpg":    true,
		"PHOTO.JPEG":   true, // case-insensitive
		"a/b/c.png":    true,
		"anim.gif":     true,
		"shot.webp":    true,
		"legacy.bmp":   true,
		"icon.svg":     true,
		"readme.txt":   false,
		"archive.zip":  false,
		"noext":        false,
		"tar.gz":       false,
		".hidden.jpg":  true, // extension still matches
		"trail.JPG.":   false,
	}
	for in, want := range cases {
		if got := IsImagePath(in); got != want {
			t.Errorf("IsImagePath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestImageMime(t *testing.T) {
	cases := map[string]string{
		"a.jpg":    "image/jpeg",
		"a.JPEG":   "image/jpeg",
		"a.png":    "image/png",
		"a.webp":   "image/webp",
		"a.svg":    "image/svg+xml",
		"a.bmp":    "image/bmp",
		"a.gif":    "image/gif",
		"readme":   "application/octet-stream",
		"file.bin": "application/octet-stream",
	}
	for in, want := range cases {
		if got := imageMime(in); got != want {
			t.Errorf("imageMime(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestReadImageSizeCap exercises the over-limit guard. We don't need a device:
// RemoteSize runs via `adb shell stat`, so a fake adb that reports a size above
// the cap short-circuits before any cat. The error must mention the cap.
//
// The real call path is `adb -s <serial> shell stat -c %s <path>`, so the fake
// adb sees $3 == "shell" (after "-s" and the serial).
func TestReadImageSizeCap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-adb shell script not run on windows CI here")
	}
	tooBig := int64(MaxImageBytes + 1)
	body := "[ \"$3\" = \"shell\" ] && echo " + itoa(tooBig) + "\n"
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)

	_, _, err := c.ReadImage(context.Background(), "dev1", "/sdcard/big.jpg")
	if err == nil {
		t.Fatal("expected size-cap error, got nil")
	}
	if !strings.Contains(err.Error(), "预览上限") {
		t.Fatalf("error should mention 预览上限, got: %v", err)
	}
}

// itoa formats a positive int64 without pulling strconv into test names.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// TestReadImageHappyPath drives the full path through a fake adb: stat reports
// a small size, then exec-out cat echoes fixed bytes. We assert the bytes and
// mime come back intact (binary-safety is exec-out's job in the real client).
//
// Dispatch is on $3: real args are "-s <serial> <subcommand> <rest>", so
// $3 is "shell" or "exec-out".
//
// NOTE: byte escapes are OCTAL (\211 = 0x89), not \xNN. Ubuntu's /bin/sh is
// dash, whose printf does not support \xNN — it would emit the literal bytes
// "\x89" and fail this test. macOS /bin/sh is bash, which does support \xNN,
// so the bug only shows on Linux CI. Octal works in both.
func TestReadImageHappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-adb shell script not run on windows CI here")
	}
	body := `case "$3" in
  shell) printf '%s' "1024" ;;
  exec-out) printf '\211PNG\015\012\032\012' ;;
esac
`
	bin := writeFakeAdb(t, body)
	c := NewClient(bin)

	out, mt, err := c.ReadImage(context.Background(), "dev1", "/sdcard/x.png")
	if err != nil {
		t.Fatal(err)
	}
	if mt != "image/png" {
		t.Fatalf("mime = %q, want image/png", mt)
	}
	want := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if string(out) != string(want) {
		t.Fatalf("bytes = % x, want % x", out, want)
	}
}
