package adb

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// TencentMirror is the preferred source (fast inside mainland China).
	tencentBase = "https://mirrors.cloud.tencent.com/AndroidSDK/"
	// googleBase is the official source, used as fallback.
	googleBase = "https://dl.google.com/android/repository/"

	downloadTimeout = 90 * time.Second
)

// supportedOS reports the adb platform-tools archive OS token for a runtime
// GOOS, or an error if unsupported.
func supportedOS(goos string) (string, error) {
	switch goos {
	case "darwin", "windows", "linux":
		return goos, nil
	}
	return "", fmt.Errorf("unsupported os: %s", goos)
}

// PlatformArchiveName returns the archive file name for the given GOOS, e.g.
// "platform-tools-latest-darwin.zip".
func PlatformArchiveName(goos string) (string, error) {
	osTok, err := supportedOS(goos)
	if err != nil {
		return "", err
	}
	return "platform-tools-latest-" + osTok + ".zip", nil
}

// DownloadURLs returns the ordered list of candidate download URLs for the
// given GOOS: a user-configured mirror first (ANDROIDFS_ADB_MIRROR), then the
// Tencent mirror, then the Google official source as fallback.
func DownloadURLs(goos string) ([]string, error) {
	archive, err := PlatformArchiveName(goos)
	if err != nil {
		return nil, err
	}
	var urls []string
	if custom := strings.TrimRight(os.Getenv("ANDROIDFS_ADB_MIRROR"), "/"); custom != "" {
		urls = append(urls, custom+"/"+archive)
	}
	urls = append(urls, tencentBase+archive)
	urls = append(urls, googleBase+archive)
	return urls, nil
}

// CacheBinaryPath returns the absolute path where the extracted adb binary
// should live inside the given cache dir (platform-tools/adb or adb.exe).
func CacheBinaryPath(cacheDir string) (string, error) {
	return filepath.Join(cacheDir, "platform-tools", binaryName()), nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "adb.exe"
	}
	return "adb"
}

// EnsureDownloaded makes sure a usable adb binary exists at CacheBinaryPath.
// If absent, it downloads the archive from the candidate sources, extracts it,
// and marks the cache ready. Existing ready caches are reused without network.
// On failure it returns an error so the caller can fall back to PATH adb.
func EnsureDownloaded(cacheDir string) (string, error) {
	binPath, err := CacheBinaryPath(cacheDir)
	if err != nil {
		return "", err
	}
	if fileExists(binPath) && readyMarkerExists(cacheDir) {
		return binPath, nil
	}

	urls, err := DownloadURLs(runtime.GOOS)
	if err != nil {
		return "", err
	}
	zipPath, err := downloadFirst(cacheDir, urls)
	if err != nil {
		return "", fmt.Errorf("download adb: %w", err)
	}
	if err := extractPlatformTools(zipPath, cacheDir); err != nil {
		return "", fmt.Errorf("extract adb: %w", err)
	}
	_ = os.Remove(zipPath)

	if !fileExists(binPath) {
		return "", errors.New("adb binary not found after extract")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("chmod adb: %w", err)
		}
	}
	if err := writeReadyMarker(cacheDir); err != nil {
		return "", fmt.Errorf("mark ready: %w", err)
	}
	return binPath, nil
}

// downloadFirst tries each URL in order, returning the saved archive path for
// the first that succeeds.
func downloadFirst(cacheDir string, urls []string) (string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	var lastErr error
	for _, u := range urls {
		archive := filepath.Base(u)
		dest := filepath.Join(cacheDir, archive)
		if err := downloadTo(u, dest); err != nil {
			lastErr = err
			continue
		}
		return dest, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no download URLs")
	}
	return "", lastErr
}

func downloadTo(url, dest string) error {
	client := &http.Client{Timeout: downloadTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(dest)
		return err
	}
	return f.Close()
}

// extractPlatformTools unzips the archive into cacheDir. The official archive
// contains a top-level "platform-tools/" directory, so entries are re-rooted
// to keep cacheDir/platform-tools as the binary home.
func extractPlatformTools(zipPath, cacheDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractEntry(f, cacheDir); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(f *zip.File, cacheDir string) error {
	// Strip a leading "platform-tools/" so files land under cacheDir/platform-tools.
	rel := strings.TrimPrefix(f.Name, "platform-tools/")
	if rel == "" || strings.HasSuffix(rel, "/") {
		return nil
	}
	dest := filepath.Join(cacheDir, "platform-tools", rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(out, rc)
	return err
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// readyMarker guards against a half-extracted cache looking complete: only a
// cache that finished download+extract+chmod carries the marker.
func readyMarkerExists(cacheDir string) bool {
	return fileExists(filepath.Join(cacheDir, ".ready"))
}

func writeReadyMarker(cacheDir string) error {
	return os.WriteFile(filepath.Join(cacheDir, ".ready"), []byte("ok"), 0o644)
}
