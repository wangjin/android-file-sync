//go:build windows

package adb

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW prevents a console subsystem child (adb.exe) from popping a
// visible cmd window. Without it, every adb invocation flashes a console.
const CREATE_NO_WINDOW = 0x08000000

// hideWindow configures the command to run without allocating a visible
// console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NO_WINDOW,
	}
}
