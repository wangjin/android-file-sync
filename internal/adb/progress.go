package adb

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"androidfs/internal/model"
)

// summaryRe matches adb's final transfer summary line, e.g.:
//
//	"/tmp/a.bin: 1 file pushed, 0 skipped. 14.9 MB/s (2000000 bytes in 0.128s)"
//
// adb 1.0.41 / platform-tools 37 does NOT stream per-chunk progress to a piped
// stderr — it only prints this one line once the transfer completes. The total
// byte count and average speed live here, so this is the source of truth for
// both the final bytes and the transfer rate. The legacy "1204/4500 (27%)"
// format this used to parse is never emitted by current adb.
var summaryRe = regexp.MustCompile(`\(([?[:digit:]]+) bytes in ([[:digit:].]+)s\)`)

// rateRe extracts the average throughput, e.g. "14.9 MB/s", from the summary.
var rateRe = regexp.MustCompile(`([[:digit:].]+)\s*(K|M|G)?B/s`)

// ParseProgress extracts transferred/total bytes and (when available) the
// average transfer speed from one stderr line of adb push/pull. Because modern
// adb only prints the final summary line, bytes == total on the single line it
// does emit, and ok is true exactly once per transfer.
//
// It returns (bytes, total, speedBytesPerSec, ok). Callers that don't care
// about speed can ignore the third value.
func ParseProgress(line string) (bytes, total, speed int64, ok bool) {
	line = strings.TrimSpace(line)
	m := summaryRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, 0, false
	}
	n := parseInt64(m[1])
	elapsed := parseFloat(m[2])
	// adb reports the total transferred, so bytes == total on the summary line.
	rate := int64(0)
	if elapsed > 0 {
		rate = int64(float64(n) / elapsed)
	}
	// Prefer the explicit "X MB/s" token when present (more precise than the
	// rounded bytes/sec derived above) — fall back to the derived rate.
	if rm := rateRe.FindStringSubmatch(line); rm != nil {
		if v := parseFloat(rm[1]); v > 0 {
			mul := 1.0
			switch rm[2] {
			case "K":
				mul = 1024
			case "M":
				mul = 1024 * 1024
			case "G":
				mul = 1024 * 1024 * 1024
			}
			rate = int64(v * mul)
		}
	}
	return n, n, rate, true
}

// RemoteSize returns the size in bytes of a file on the device, used to set a
// real Total for pull before the transfer begins (adb gives no streaming
// progress, so the bar would otherwise stay at 0% until the final summary).
// Returns an error if stat fails or the path is not a regular file.
func (c *AdbClient) RemoteSize(ctx context.Context, serial, remote string) (int64, error) {
	out, err := c.Shell(ctx, serial, "stat -c %s "+quoteArg(remote))
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", remote, err)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, fmt.Errorf("stat %s: empty", remote)
	}
	return parseInt64(out), nil
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

// parseFloat parses a base-10 decimal (no exponent) such as "0.128" or "14.9".
// Returns 0 for empty or non-numeric input.
func parseFloat(s string) float64 {
	var whole, frac float64
	var div float64 = 1
	dot := false
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			if dot {
				div *= 10
				frac = frac*10 + float64(c-'0')
			} else {
				whole = whole*10 + float64(c-'0')
			}
		case c == '.':
			dot = true
		default:
			// stop at first non-numeric char (e.g. the space in "0.128s")
			if whole != 0 || frac != 0 || dot {
				// already consumed digits, finish
			}
			goto done
		}
	}
done:
	if div == 0 {
		div = 1
	}
	return whole + frac/div
}
