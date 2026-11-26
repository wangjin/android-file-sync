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
// CPU: we read /proc/stat twice inside a single adb shell command (with a
// sleep in between) and diff the cumulative jiffies per core to get both a
// per-core and an overall utilisation. If /proc/stat is unusable we fall back
// to a one-shot `top` snapshot (overall only). NPU: we read Rockchip's
// debugfs counter /sys/kernel/debug/rknpu/load, which reports per-core busy
// percentages. All are best-effort: any device without the corresponding
// command/node simply yields nil metrics so the frontend can show "n/a"
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
// capturing the core INDEX and its load value (never the index alone), e.g.
// "NPU load:  Core0:  40%, Core1:  0%, Core2:  75%," -> (0,40) (1,0) (2,75).
var npuCoreRe = regexp.MustCompile(`(?i)core\s*(\d+)\s*:\s*(\d+(?:\.\d+)?)`)

// npuPctRe matches standalone percentage values such as "42%".
var npuPctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// npuBareRe is a last resort for nodes that print a bare number like "42".
var npuBareRe = regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)

// procStatRe matches the cpu summary and per-core lines of /proc/stat:
// "cpu  12 34 ..." and "cpu0 12 34 ...". Column meaning is positional:
// user nice system idle iowait irq softirq steal guest guest_nice.
var procStatRe = regexp.MustCompile(`^cpu(\d*)\s+([0-9 ]+)\s*$`)

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

// SampleCPUCores returns the device's overall CPU utilisation (0-100) plus
// per-core values (index == core number) by reading /proc/stat twice inside a
// SINGLE adb shell command with a `sleep` in between — one connection per
// sample, the same cost as the old `top` snapshot. Per-core utilisation must
// come from the diff of two cumulative-jiffies frames; a single frame is
// useless. sleep bounds the measurement window and must stay well under the
// sample interval so the ticker is never starved.
//
// If /proc/stat is unavailable or unparseable we fall back to the legacy
// `top` overall path (cores == nil) so the panel keeps working on exotic
// devices.
func (c *AdbClient) SampleCPUCores(ctx context.Context, serial string, sleep time.Duration) (float64, []float64, error) {
	if sleep < 100*time.Millisecond {
		sleep = 100 * time.Millisecond
	}
	if sleep > time.Second {
		sleep = time.Second
	}
	// A quoted marker between the two cats makes frame splitting
	// unambiguous (the /proc/stat body itself is many lines long). Quotes
	// are required: an unquoted leading # would start a shell comment.
	cmd := fmt.Sprintf("cat /proc/stat 2>/dev/null; echo '%s'; sleep %s; cat /proc/stat 2>/dev/null",
		procStatMarker, formatSleep(sleep))
	out, err := c.Shell(ctx, serial, cmd)
	if err == nil {
		if a, b, found := splitProcStat(out); found {
			if overall, cores, ok := parseProcStatDiff(a, b); ok {
				return overall, cores, nil
			}
		}
	}
	// Fallback: legacy top-based overall reading (no per-core data).
	overall, ferr := c.SampleCPU(ctx, serial)
	if ferr != nil {
		return 0, nil, fmt.Errorf("proc/stat and top fallback both failed: %w", ferr)
	}
	return overall, nil, nil
}

const procStatMarker = "###AFS-PROCSTAT-SPLIT###"

// formatSleep renders a duration for toybox `sleep` (fractional seconds).
func formatSleep(d time.Duration) string {
	s := fmt.Sprintf("%.2f", d.Seconds())
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "0" {
		s = "0.1"
	}
	return s
}

// splitProcStat splits the combined two-cat output at the marker echo.
func splitProcStat(out string) (a, b string, ok bool) {
	i := strings.Index(out, procStatMarker)
	if i < 0 {
		return "", "", false
	}
	return out[:i], out[i+len(procStatMarker):], true
}

// parseProcStatDiff diffs two /proc/stat frames and returns the overall and
// per-core CPU utilisation (0-100). Frames are aligned by the numeric cpuN
// index so core order never depends on output order. busy = total - (idle +
// iowait); usage = 100 * Δbusy / Δtotal, clamped. A core (or the aggregate)
// whose Δtotal is 0 — the two frames read identical counters — is unusable.
//
// The returned cores slice is ordered ascending from cpu0 and contiguous; if
// a core in the middle is unusable, or no core data exists at all, cores is
// nil while overall may still be valid (it then comes from the "cpu"
// aggregate row, or the mean of the core values when the row is absent).
func parseProcStatDiff(a, b string) (overall float64, cores []float64, ok bool) {
	frameA := parseProcStatFrame(a)
	frameB := parseProcStatFrame(b)
	if len(frameA) == 0 || len(frameB) == 0 {
		return 0, nil, false
	}

	// Per-core diffs, aligned by numeric index across both frames. We scan
	// the full union range 0..maxIdx so a core missing from one frame
	// (hotplug, truncated output) only marks itself unusable instead of
	// truncating every higher core.
	type coreDiff struct {
		val    float64
		viable bool
	}
	maxIdx := -1
	for idx := range frameA {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	for idx := range frameB {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	var coreDiffs []coreDiff
	for idx := 0; idx <= maxIdx; idx++ {
		ca, oka := frameA[idx]
		cb, okb := frameB[idx]
		// float64 deltas: unsigned jiffy subtraction would wrap (and thus
		// silently pass the >0 guard) if counters reset between frames.
		dt := float64(statTotal(cb)) - float64(statTotal(ca))
		if !oka || !okb || dt <= 0 || len(ca) < 4 || len(cb) < 4 {
			coreDiffs = append(coreDiffs, coreDiff{viable: false})
			continue
		}
		db := dt - (float64(statIdle(cb)) - float64(statIdle(ca)))
		coreDiffs = append(coreDiffs, coreDiff{val: clampPct(100 * db / dt), viable: true})
	}

	// Cores are only reported when cpu0..cpuN are contiguous and all viable;
	// a hole in the middle would shift positional indexing, so we give up on
	// per-core data instead of emitting misaligned values.
	usable := 0
	for _, d := range coreDiffs {
		if d.viable {
			usable++
		}
	}
	if len(coreDiffs) > 0 && usable == len(coreDiffs) {
		cores = make([]float64, len(coreDiffs))
		for i, d := range coreDiffs {
			cores[i] = d.val
		}
	}

	// Overall: the "cpu" aggregate row when usable...
	if ca, ka := frameA[-1]; ka {
		if cb, kb := frameB[-1]; kb {
			dt := float64(statTotal(cb)) - float64(statTotal(ca))
			if dt > 0 && len(ca) >= 4 && len(cb) >= 4 {
				db := dt - (float64(statIdle(cb)) - float64(statIdle(ca)))
				return clampPct(100 * db / dt), cores, true
			}
		}
	}
	// ...otherwise the mean of the viable cores.
	if usable > 0 {
		sum := 0.0
		n := 0
		for _, d := range coreDiffs {
			if d.viable {
				sum += d.val
				n++
			}
		}
		return clampPct(sum / float64(n)), cores, true
	}
	return 0, nil, false
}

// parseProcStatFrame indexes one /proc/stat text by core number: -1 for the
// aggregate "cpu" row, 0..N for "cpuN" rows. Values are cumulative jiffies.
func parseProcStatFrame(text string) map[int][]uint64 {
	frame := map[int][]uint64{}
	for _, raw := range strings.Split(text, "\n") {
		m := procStatRe.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		idx := -1 // aggregate "cpu" row
		if m[1] != "" {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			idx = n
		}
		var cols []uint64
		for _, c := range strings.Fields(m[2]) {
			v, err := strconv.ParseUint(c, 10, 64)
			if err != nil {
				break
			}
			cols = append(cols, v)
		}
		frame[idx] = cols
	}
	return frame
}

// statTotal sums all counter columns (Σ of the positional stat fields).
func statTotal(cols []uint64) uint64 {
	var t uint64
	for _, v := range cols {
		t += v
	}
	return t
}

// statIdle sums the idle and iowait columns (indices 3 and 4). Missing
// trailing columns simply contribute nothing.
func statIdle(cols []uint64) uint64 {
	var t uint64
	if len(cols) > 3 {
		t += cols[3]
	}
	if len(cols) > 4 {
		t += cols[4]
	}
	return t
}

// SampleNPULoads reads /sys/kernel/debug/rknpu/load once and returns the
// overall NPU load (0-100, the mean of the cores when per-core values exist)
// plus per-core values (index == core number). Returns an error when the node
// is missing (non-Rockchip SoC) or unparseable.
func (c *AdbClient) SampleNPULoads(ctx context.Context, serial string) (float64, []float64, error) {
	out, err := c.Shell(ctx, serial, "cat /sys/kernel/debug/rknpu/load 2>/dev/null")
	if err != nil {
		return 0, nil, fmt.Errorf("rknpu/load: %w", err)
	}
	overall, cores, ok := parseNPULoads(out)
	if !ok {
		return 0, nil, fmt.Errorf("rknpu/load: no value found in output %q", strings.TrimSpace(out))
	}
	return overall, cores, nil
}

// parseNPULoads pulls per-core NPU loads out of the rknpu load node. The
// RK3588 prints "NPU load:  Core0:  0%, Core1:  0%, Core2:  0%," — note the
// core INDICES (0/1/2) are also digits, so we pair each index with its value
// explicitly. overall is the mean of the cores. When no CoreN labels exist
// we keep the legacy scalar semantics: cores == nil, overall == max of any
// percentages found (or a single bare number).
func parseNPULoads(raw string) (overall float64, cores []float64, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil, false
	}

	// 1) Per-core entries: "Core0: 40%, Core1: 0%, Core2: 75%"
	if ms := npuCoreRe.FindAllStringSubmatch(trimmed, -1); len(ms) > 0 {
		vals := map[int]float64{}
		maxIdx := -1
		for _, m := range ms {
			idx, err1 := strconv.Atoi(m[1])
			v, err2 := strconv.ParseFloat(m[2], 64)
			if err1 != nil || err2 != nil || v < 0 || v > 100 {
				continue
			}
			vals[idx] = v
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		if len(vals) > 0 {
			// Index=core-number only makes sense when 0..maxIdx are all
			// present; otherwise fall through to the scalar semantics.
			if len(vals) == maxIdx+1 {
				cores = make([]float64, maxIdx+1)
				sum := 0.0
				for i := 0; i <= maxIdx; i++ {
					cores[i] = clampPct(vals[i])
					sum += cores[i]
				}
				return clampPct(sum / float64(len(cores))), cores, true
			}
		}
	}

	// 2) Standalone percentages: "load: 42%"
	if ms := npuPctRe.FindAllStringSubmatch(trimmed, -1); len(ms) > 0 {
		if v, ok := maxInRange(ms); ok {
			return clampPct(v), nil, true
		}
	}

	// 3) A single bare number (node that prints just "42").
	if ms := npuBareRe.FindAllStringSubmatch(trimmed, -1); len(ms) == 1 {
		if v, err := strconv.ParseFloat(ms[0][1], 64); err == nil {
			return clampPct(v), nil, true
		}
	}

	return 0, nil, false
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
