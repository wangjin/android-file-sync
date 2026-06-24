package adb

import (
	"os"
	"path/filepath"
	"runtime"
)

// EmbeddedBinary returns the absolute path to the platform-appropriate adb
// binary committed under build/adb/<os>-<arch>/. It does NOT extract anything
// at runtime in MVP — the binaries ship alongside and are resolved by the
// build environment. Returns the binary name "adb" as a fallback so system
// PATH adb is used during local dev when the committed binary is absent.
func EmbeddedBinary() string {
	name := "adb"
	if runtime.GOOS == "windows" {
		name = "adb.exe"
	}
	rel := filepath.Join("build", "adb", runtime.GOOS+"-"+runtime.GOARCH, name)
	if abs, err := filepath.Abs(rel); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	return name // fallback to PATH
}
