package main

import (
	"os"
	"runtime"
)

func runtimeGOOS() string { return runtime.GOOS }

func hostHome() (string, error) { return os.UserHomeDir() }
