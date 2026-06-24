package adb

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformArchiveName(t *testing.T) {
	cases := []struct {
		goos string
		want string
	}{
		{"darwin", "platform-tools-latest-darwin.zip"},
		{"windows", "platform-tools-latest-windows.zip"},
		{"linux", "platform-tools-latest-linux.zip"},
	}
	for _, c := range cases {
		got, err := PlatformArchiveName(c.goos)
		if err != nil {
			t.Fatalf("PlatformArchiveName(%q) error: %v", c.goos, err)
		}
		if got != c.want {
			t.Errorf("PlatformArchiveName(%q) = %q want %q", c.goos, got, c.want)
		}
	}
}

func TestPlatformArchiveNameUnsupported(t *testing.T) {
	if _, err := PlatformArchiveName("freebsd"); err == nil {
		t.Fatal("expected error for unsupported os freebsd")
	}
}

func TestDownloadURLs(t *testing.T) {
	urls, err := DownloadURLs("linux")
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) < 2 {
		t.Fatalf("expected at least 2 candidate URLs, got %d", len(urls))
	}
	// Tencent mirror must be first (preferred for CN users).
	if !strings.HasPrefix(urls[0], "https://mirrors.cloud.tencent.com/AndroidSDK/") {
		t.Errorf("first URL should be Tencent mirror, got %q", urls[0])
	}
	// Every URL must name the linux archive.
	for _, u := range urls {
		if !strings.HasSuffix(u, "platform-tools-latest-linux.zip") {
			t.Errorf("URL %q does not target linux archive", u)
		}
	}
	// Google official must be present as fallback.
	foundGoogle := false
	for _, u := range urls {
		if strings.Contains(u, "dl.google.com") {
			foundGoogle = true
		}
	}
	if !foundGoogle {
		t.Error("Google official fallback URL missing")
	}
}

func TestDownloadURLsCustomMirror(t *testing.T) {
	t.Setenv("ANDROIDFS_ADB_MIRROR", "https://example.test/mirror/")
	urls, err := DownloadURLs("linux")
	if err != nil {
		t.Fatal(err)
	}
	if urls[0] != "https://example.test/mirror/platform-tools-latest-linux.zip" {
		t.Errorf("custom mirror not first: %q", urls[0])
	}
}

func TestCacheBinaryPath(t *testing.T) {
	dir := t.TempDir()
	p, err := CacheBinaryPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(p)
	want := "adb"
	if runtime.GOOS == "windows" {
		want = "adb.exe"
	}
	if name != want {
		t.Errorf("basename = %q want %q", name, want)
	}
	if !strings.HasPrefix(p, dir) {
		t.Errorf("path %q not under cache dir %q", p, dir)
	}
}
