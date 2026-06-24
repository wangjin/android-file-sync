package adb

import (
	"regexp"
	"strings"

	"androidfs/internal/model"
)

// progressRe matches adb push/pull lines like "1204/4500 (27%)".
var progressRe = regexp.MustCompile(`^(\d+)/(\d+)\s+\(\d+%\)$`)

// ParseProgress extracts transferred/total bytes from one stderr line of adb
// push/pull. ok is false when the line is not a progress line.
func ParseProgress(line string) (bytes, total int64, ok bool) {
	line = strings.TrimSpace(line)
	m := progressRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	return parseInt64(m[1]), parseInt64(m[2]), true
}

// deviceRe splits each "adb devices -l" body row.
var deviceRe = regexp.MustCompile(`^(\S+)\s+(\S+)(.*)$`)

// modelKeyRe extracts the model name from the device-line key/value suffix.
var modelKeyRe = regexp.MustCompile(`model:(\S+)`)

// ParseDevices parses the stdout of `adb devices -l` into Device values.
// Rows that are not "device"/"offline"/"unauthorized" still appear with their
// raw state; transport is inferred from the serial (ip:port => tcpip).
func ParseDevices(output string) []model.Device {
	var devs []model.Device
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") {
			continue
		}
		m := deviceRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		serial, state := m[1], m[2]
		rest := m[3]
		d := model.Device{
			Serial:    serial,
			State:     state,
			Transport: "usb",
		}
		if strings.Contains(serial, ":") {
			d.Transport = "tcpip"
		}
		if mm := modelKeyRe.FindStringSubmatch(rest); mm != nil {
			d.Model = mm[1]
		}
		devs = append(devs, d)
	}
	return devs
}

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
