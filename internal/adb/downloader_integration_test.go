//go:build integration

package adb

import (
	"runtime"
	"testing"
)

// TestEnsureDownloadedLive verifies the full download+extract path against the
// real Tencent mirror. Guarded by the "integration" build tag so the normal
// test suite stays offline and fast. Run with:
//
//	go test -tags=integration ./internal/adb/... -run TestEnsureDownloadedLive -v
func TestEnsureDownloadedLive(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("live extract test runs on unix only")
	}
	dir := t.TempDir()
	binPath, err := EnsureDownloaded(dir)
	if err != nil {
		t.Fatalf("EnsureDownloaded failed: %v", err)
	}
	if !fileExists(binPath) {
		t.Fatalf("adb not present at %s", binPath)
	}
	if !readyMarkerExists(dir) {
		t.Fatal("ready marker not written")
	}
}
