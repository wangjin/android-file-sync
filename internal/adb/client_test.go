package adb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFakeAdb creates an executable that prints its args-derived output.
// We emulate `adb devices -l` by detecting the "devices" argument.
func writeFakeAdb(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	name := "fake-adb"
	if runtime.GOOS == "windows" {
		name = "fake-adb.bat"
	}
	p := filepath.Join(dir, name)
	script := "#!/bin/sh\n" + body
	if runtime.GOOS == "windows" {
		script = "@echo off\n" + body
	}
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListDevices(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-adb shell script not run on windows CI here")
	}
	out := `echo 'List of devices attached
emulator-5554   device product:x model:Pixel_5
'`
	bin := writeFakeAdb(t, out)
	c := NewClient(bin)
	devs, err := c.ListDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 || devs[0].Serial != "emulator-5554" {
		t.Fatalf("got %+v", devs)
	}
}
