//go:build !windows

package adb

import "os/exec"

// hideWindow is a no-op on non-Windows platforms: unix processes don't spawn
// a visible console window the way Windows console binaries do.
func hideWindow(*exec.Cmd) {}
