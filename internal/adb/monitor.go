package adb

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// This file samples device resource usage for the live monitor panel.
//
// CPU: we ask the device for a one-shot `top` snapshot and parse the
// overall utilization out of it. NPU: we read Rockchip's debugfs counter
// /sys/kernel/debug/rknpu/load, which reports the NPU cores' busy
// percentage. Both are best-effort: any device without the corresponding
// command/node simply yields a nil metric so the frontend can show "n/a"
// instead of breaking the whole panel.

// sampleTimeout bounds a single sample so a hung adb call can't stall the
// ticker forever.
const sampleTimeout = 5 * time.Second

// cpuIdleRe matches "94.4 id" (number before label) in procps top output,
// e.g. "%Cpu(s):  3.2 us,  1.8 sy,  0.0 ni, 94.4 id,  0.5 wa".
var cpuIdleRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s+id\b`)

// androidUserRe matches the Android toybox top summary line:
// "User 12%, System 5%, IOW 0%, IRQ 0%"
var androidCPULineRe = regexp.MustCompile(`(?i)User\s+([0-9.]+)%.*?System\s+([0-9.]+)%(?:.*?IOW\s+([0-9.]+)%)?(?:.*?IRQ\s+([0-9.]+)%)?`)

// npuPctRe grabs the first integer/decimal that is followed by an optional %
// — the rknpu load node reports its value as a bare number or "N%".
var npuNumRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%?`)

// SampleCPU returns the device's overall CPU utilisation (0-100) by running a
// single `top -bn1` snapshot and parsing the CPU summary line. Returns an
// error if the command fails or no parseable summary is found (the caller
// then reports the metric as unavailable).
func (c *AdbClient) SampleCPU(ctx context.Context, serial string) (float64, error) {
	out, err := c.Shell(ctx, serial, "top -bn1 2>/dev/null || top -n1 2>/dev/null || top -b -n 1 2>/dev/null")
	if err != nil {
		return 0, fmt.Errorf("top: %w", err)
	}
	pct, ok := parseTopCPU(out)
	if !ok {
		return 0, fmt.Errorf("top: no parseable CPU summary in output")
	}
	return pct, nil
}

// parseTopCPU extracts overall CPU utilisation from `top` output. It handles
// two common formats:
//   - procps: "%Cpu(s):  3.2 us,  1.8 sy,  0.0 ni, 94.4 id, ..."  → 100-idle
//   - Android toybox: "User 12%, System 5%, IOW 0%, IRQ 0%"       → sum
func parseTopCPU(output string) (float64, bool) {
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		lower := strings.ToLower(line)

		// procps-ng style: "%Cpu(s):  3.2 us,  1.8 sy,  0.0 ni, 94.4 id, ..."
		// Here the number precedes the label: "94.4 id".
		if strings.Contains(lower, "cpu") {
			if m := cpuIdleRe.FindStringSubmatch(line); m != nil {
				if idle, err := strconv.ParseFloat(m[1], 64); err == nil {
					return clampPct(100 - idle), true
				}
			}
		}

		// Android / toybox: "User 12%, System 5%, IOW 0%, IRQ 0%"
		if strings.Contains(lower, "user") && strings.Contains(lower, "system") {
			if m := androidCPULineRe.FindStringSubmatch(line); m != nil {
				user := parseFloatField(m[1])
				system := parseFloatField(m[2])
				iow := parseFloatField(m[3])
				irq := parseFloatField(m[4])
				return clampPct(user + system + iow + irq), true
			}
		}

		// Fallback: look for a bare "idle NN" or "idle: NN" pattern.
		if m := cpuIdleRe.FindStringSubmatch(line); m != nil {
			if idle, err := strconv.ParseFloat(m[1], 64); err == nil {
				return clampPct(100 - idle), true
			}
		}
	}
	return 0, false
}

func parseFloatField(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// SampleNPULoad reads /sys/kernel/debug/rknpu/load and returns the NPU busy
// percentage. On multi-core NPUs the node may report per-core values; we take
// the max so a single busy core still shows a meaningful number. Returns an
// error when the node is missing (non-Rockchip SoC) or unparseable.
func (c *AdbClient) SampleNPULoad(ctx context.Context, serial string) (float64, error) {
	out, err := c.Shell(ctx, serial, "cat /sys/kernel/debug/rknpu/load 2>/dev/null")
	if err != nil {
		return 0, fmt.Errorf("rknpu/load: %w", err)
	}
	pct, ok := parseNPULoad(out)
	if !ok {
		return 0, fmt.Errorf("rknpu/load: no value found in output %q", strings.TrimSpace(out))
	}
	return pct, nil
}

// parseNPULoad pulls the NPU load percentage out of the rknpu load node.
func parseNPULoad(raw string) (float64, bool) {
	matches := npuNumRe.FindAllStringSubmatch(strings.TrimSpace(raw), -1)
	if len(matches) == 0 {
		return 0, false
	}
	max := 0.0
	for _, m := range matches {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		// Guard against parsing an unrelated big number (e.g. a pid): the NPU
		// load is a 0-100 percentage, so ignore out-of-range values.
		if v < 0 || v > 100 {
			continue
		}
		if v > max {
			max = v
		}
	}
	return clampPct(max), true
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
