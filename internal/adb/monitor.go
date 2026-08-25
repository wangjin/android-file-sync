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

// cpuIdleRe matches "94.4 id" (number before label) in classic procps top
// output, e.g. "%Cpu(s):  3.2 us,  1.8 sy,  0.0 ni, 94.4 id,  0.5 wa".
var cpuIdleRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s+id\b`)

// androidCPULineRe matches the Android toybox summary line:
// "User 12%, System 5%, IOW 0%, IRQ 0%"
var androidCPULineRe = regexp.MustCompile(`(?i)User\s+([0-9.]+)%.*?System\s+([0-9.]+)%(?:.*?IOW\s+([0-9.]+)%)?(?:.*?IRQ\s+([0-9.]+)%)?`)

// RK3588 (and other toybox) top prints an aggregate line where the number
// PRECEDES the label and the whole thing is scaled to N cores:
// "800%cpu  42%user  0%nice  46%sys 704%idle  0%iow  8%irq  0%sirq  0%host"
// total == sum of all fields (e.g. 800 for 8 cores), so overall usage is
// 100 * (1 - idle/total).
var toyboxTotalRe = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*%cpu\b`)
var toyboxIdleRe = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*%idle\b`)

// npuCoreRe matches per-core entries in /sys/kernel/debug/rknpu/load,
// capturing ONLY the load value and not the core index, e.g.
// "NPU load:  Core0:  40%, Core1:  0%, Core2:  75%," -> 40, 0, 75.
var npuCoreRe = regexp.MustCompile(`(?i)core\s*\d+\s*:\s*(\d+(?:\.\d+)?)`)

// npuPctRe matches standalone percentage values such as "42%".
var npuPctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// npuBareRe is a last resort for nodes that print a bare number like "42".
var npuBareRe = regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)

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
// several formats:
//   - RK3588/toybox aggregate: "800%cpu  42%user ... 704%idle ..."
//     (numbers precede labels, scaled to N cores) -> 100*(1-idle/total)
//   - procps-ng: "%Cpu(s):  3.2 us, ... 94.4 id, ..." -> 100-idle
//   - Android toybox: "User 12%, System 5%, IOW 0%, IRQ 0%" -> sum of busy
func parseTopCPU(output string) (float64, bool) {
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		lower := strings.ToLower(line)

		// 1) RK3588 / toybox aggregate line: "800%cpu  42%user ... 704%idle ..."
		if t := toyboxTotalRe.FindStringSubmatch(line); t != nil {
			if i := toyboxIdleRe.FindStringSubmatch(line); i != nil {
				if total, err1 := strconv.ParseFloat(t[1], 64); err1 == nil && total > 0 {
					if idle, err2 := strconv.ParseFloat(i[1], 64); err2 == nil {
						return clampPct(100 * (1 - idle/total)), true
					}
				}
			}
		}

		// 2) procps-ng style: "%Cpu(s):  3.2 us, ... 94.4 id, ..."
		if strings.Contains(lower, "cpu") {
			if m := cpuIdleRe.FindStringSubmatch(line); m != nil {
				if idle, err := strconv.ParseFloat(m[1], 64); err == nil {
					return clampPct(100 - idle), true
				}
			}
		}

		// 3) Android/toybox: "User 12%, System 5%, IOW 0%, IRQ 0%"
		if strings.Contains(lower, "user") && strings.Contains(lower, "system") {
			if m := androidCPULineRe.FindStringSubmatch(line); m != nil {
				return clampPct(parseFloatField(m[1]) + parseFloatField(m[2]) +
					parseFloatField(m[3]) + parseFloatField(m[4])), true
			}
		}

		// 4) Fallback: a bare "NN id" pattern on any line.
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

// parseNPULoad pulls the NPU load percentage out of the rknpu load node. The
// RK3588 prints "NPU load:  Core0:  0%, Core1:  0%, Core2:  0%," — note the
// core INDICES (0/1/2) are also digits, so we must parse the per-core load
// value specifically rather than grabbing any number. We take the max across
// cores so a single busy core still shows a meaningful number.
func parseNPULoad(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}

	// 1) Per-core entries: "Core0: 40%, Core1: 0%, Core2: 75%"
	if ms := npuCoreRe.FindAllStringSubmatch(trimmed, -1); len(ms) > 0 {
		if v, ok := maxInRange(ms); ok {
			return clampPct(v), true
		}
	}

	// 2) Standalone percentages: "load: 42%"
	if ms := npuPctRe.FindAllStringSubmatch(trimmed, -1); len(ms) > 0 {
		if v, ok := maxInRange(ms); ok {
			return clampPct(v), true
		}
	}

	// 3) A single bare number (node that prints just "42").
	if ms := npuBareRe.FindAllStringSubmatch(trimmed, -1); len(ms) == 1 {
		if v, err := strconv.ParseFloat(ms[0][1], 64); err == nil {
			return clampPct(v), true
		}
	}

	return 0, false
}

// maxInRange returns the largest value in [0,100] among the capture groups of
// the given regex matches, so stray out-of-range digits (core indices, pids,
// ...) are ignored.
func maxInRange(ms [][]string) (float64, bool) {
	max := -1.0
	for _, m := range ms {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil || v < 0 || v > 100 {
			continue
		}
		if v > max {
			max = v
		}
	}
	if max < 0 {
		return 0, false
	}
	return max, true
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
