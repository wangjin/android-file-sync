package adb

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseTopCPU_ProcpsFormat(t *testing.T) {
	out := `top - 14:23:45 up 3 days,  2:34,  0 users,  load average: 0.10, 0.05, 0.01
Tasks: 120 total,   1 running, 119 sleeping,   0 stopped,   0 zombie
%Cpu(s):  3.2 us,  1.8 sy,  0.0 ni, 94.4 id,  0.5 wa,  0.0 hi,  0.1 si,  0.0 st
MiB Mem :   3918.5 total,    345.1 free,   3456.7 used,    121.6 buff/cache
MiB Swap:  2048.0 total,    512.0 free,   1536.0 used.    743.2 avail Mem
`
	cpu, ok := parseTopCPU(out)
	if !ok {
		t.Fatal("expected to parse procps top output")
	}
	if cpu < 4 || cpu > 7 {
		t.Fatalf("unexpected CPU: %v", cpu)
	}
}

func TestParseTopCPU_AndroidSummaryLine(t *testing.T) {
	out := `User 12%, System 5%, IOW 0%, IRQ 0%
User: 60/284:  3.6%, Sys: 20/284:  1.3%, Nick: 0/284:  0.0%, Idle: 204/284: 71.8%
  PID   PPID  %CPU  VSIZE  RSS  WCHAN  ADDR  PCY PPSW  Name
    1     0   0.0  5120  320  wait   0     0   -    0  init
`
	cpu, ok := parseTopCPU(out)
	if !ok {
		t.Fatal("expected to parse Android-style top output")
	}
	if cpu < 16.9 || cpu > 17.1 {
		t.Fatalf("unexpected CPU: %v (want ~17)", cpu)
	}
}

func TestParseTopCPU_NoSummary(t *testing.T) {
	out := `
  PID   PPID  %CPU  VSIZE  RSS  WCHAN  ADDR  PCY PPSW  Name
    1     0   0.0  5120  320  wait   0     0   -    0  init
`
	if _, ok := parseTopCPU(out); ok {
		t.Fatal("expected no parse for process-list-only output")
	}
}

func TestParseTopCPU_RK3588ToyboxAggregate(t *testing.T) {
	// Real output from RK3588 (8-core) via `top -bn1` on Android toybox.
	out := `top: unknown option 'b'
Usage: top [OPTION...]
top - 07:58:03 up 49 min,  0 users,  load average: 0.05, 0.09, 0.10
Tasks:  203 total,   1 running, 202 sleeping,   0 stopped,   0 zombie
800%cpu  42%user  0%nice 46%sys 704%idle  0%iow  8%irq  0%sirq  0%host
 3.8G Mem  1.6G free  2.2G used,   0 buffers
 0.0KiB Swap  0.0KiB free   0.0KiB used  3.6G cached
`
	cpu, ok := parseTopCPU(out)
	if !ok {
		t.Fatal("expected to parse RK3588 toybox aggregate top output")
	}
	// 100 * (1 - 704/800) = 12
	if cpu < 11.9 || cpu > 12.1 {
		t.Fatalf("unexpected CPU: %v (want 12)", cpu)
	}
}

func TestParseTopCPU_ClaampsToRange(t *testing.T) {
	if got := clampPct(-5); got != 0 {
		t.Errorf("clampPct(-5) = %v, want 0", got)
	}
	if got := clampPct(120); got != 100 {
		t.Errorf("clampPct(120) = %v, want 100", got)
	}
	if got := clampPct(42); got != 42 {
		t.Errorf("clampPct(42) = %v, want 42", got)
	}
}

func TestParseNPULoads(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		want      float64
		wantCores []float64
		ok        bool
	}{
		// Real RK3588 sample: three cores with DIFFERENT values, so the
		// per-core array and the mean can't be confused with the old max.
		{"rk3588 per core", "NPU load:  Core0:  40%, Core1:  0%, Core2:  75%.\n", 115.0 / 3.0, []float64{40, 0, 75}, true},
		{"multi core", "Core 0: 10%\nCore 1: 45%\nCore 2: 0%\n", 55.0 / 3.0, []float64{10, 45, 0}, true},
		{"all idle cores", "NPU load: Core0: 0%, Core1: 0%, Core2: 0%.", 0, []float64{0, 0, 0}, true},
		{"plain percent", "42\n", 42, nil, true},
		{"single core load label", "load: 25%\n", 25, nil, true},
		{"no percent sign", "37\n", 37, nil, true},
		// Core index hole (no Core1) → scalar semantics, max of percents.
		{"core index gap", "Core0: 10%, Core2: 30%", 30, nil, true},
		{"empty", "", 0, nil, false},
		{"garbage", "N/A", 0, nil, false},
	}
	for _, tc := range cases {
		overall, cores, ok := parseNPULoads(tc.in)
		if ok != tc.ok {
			t.Fatalf("%s: ok=%v want %v", tc.name, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if diff := overall - tc.want; diff > 0.01 || diff < -0.01 {
			t.Errorf("%s: overall %v want %v", tc.name, overall, tc.want)
		}
		if len(cores) != len(tc.wantCores) {
			t.Fatalf("%s: cores %v want %v", tc.name, cores, tc.wantCores)
		}
		for i := range cores {
			if cores[i] != tc.wantCores[i] {
				t.Errorf("%s: cores[%d]=%v want %v", tc.name, i, cores[i], tc.wantCores[i])
			}
		}
	}
}

// procStatFrameA/B build an 8-core /proc/stat pair. In frame B core k burned
// 10k% busy over a 100-jiffy window (idle absorbed the rest), and the
// aggregate row is the exact column-wise sum.
func procStatFrame(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func statCore(user, idle uint64) string {
	return fmt.Sprintf("%d 0 0 %d 0 0 0 0", user, idle)
}

func TestParseProcStatDiff_EightCores(t *testing.T) {
	var a, b []string
	a = append(a, "cpu  0 0 0 6400 0 0 0 0")
	b = append(b, "cpu  280 0 0 6920 0 0 0 0")
	for k := 0; k < 8; k++ {
		a = append(a, fmt.Sprintf("cpu%d  %s", k, statCore(0, 800)))
		b = append(b, fmt.Sprintf("cpu%d  %s", k, statCore(uint64(10*k), uint64(900-10*k))))
	}
	overall, cores, ok := parseProcStatDiff(procStatFrame(a), procStatFrame(b))
	if !ok {
		t.Fatal("expected 8-core frames to parse")
	}
	if len(cores) != 8 {
		t.Fatalf("expected 8 cores, got %d", len(cores))
	}
	for k, v := range cores {
		want := float64(10 * k)
		if diff := v - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("cpu%d = %v, want %v", k, v, want)
		}
	}
	// Aggregate: db=280, dt=800 → 35%.
	if diff := overall - 35; diff > 0.01 || diff < -0.01 {
		t.Errorf("overall = %v, want 35", overall)
	}
}

func TestParseProcStatDiff_ZeroDeltaCoreDropsArray(t *testing.T) {
	// cpu3 counters did not advance at all (Δtotal=0) → that core is
	// unusable, so per-core data is dropped (a hole in the array would
	// misalign indices) while the overall stays valid.
	aLines := []string{"cpu  0 0 0 6400 0 0 0 0"}
	bLines := []string{"cpu  250 0 0 6850 0 0 0 0"}
	for k := 0; k < 8; k++ {
		aLines = append(aLines, fmt.Sprintf("cpu%d  %s", k, statCore(0, 800)))
		if k == 3 {
			bLines = append(bLines, fmt.Sprintf("cpu%d  %s", k, statCore(0, 800)))
		} else {
			bLines = append(bLines, fmt.Sprintf("cpu%d  %s", k, statCore(uint64(10*k), uint64(900-10*k))))
		}
	}
	overall, cores, ok := parseProcStatDiff(procStatFrame(aLines), procStatFrame(bLines))
	if !ok {
		t.Fatal("expected overall to parse despite frozen core")
	}
	if cores != nil {
		t.Fatalf("expected nil cores with a Δtotal=0 hole, got %v", cores)
	}
	// Aggregate: db=250, dt=700 → 35.714...%
	if diff := overall - 250.0/7.0; diff > 0.01 || diff < -0.01 {
		t.Errorf("overall = %v, want %v", overall, 250.0/7.0)
	}
}

func TestParseProcStatDiff_MissingColumns(t *testing.T) {
	// iowait (col 5) absent → idle column alone is used.
	frameA := "cpu0  0 0 0 800\n"
	frameB := "cpu0  30 0 0 870\n"
	overall, cores, ok := parseProcStatDiff(frameA, frameB)
	if !ok {
		t.Fatal("expected 4-column frame to parse")
	}
	if len(cores) != 1 || cores[0] != 30 {
		t.Fatalf("cores = %v, want [30]", cores)
	}
	if diff := overall - 30; diff > 0.01 || diff < -0.01 {
		t.Errorf("overall = %v, want 30", overall)
	}

	// Fewer than 4 columns: no idle column at all → nothing usable.
	if _, _, ok := parseProcStatDiff("cpu0  5 5 5\n", "cpu0  10 10 10\n"); ok {
		t.Fatal("expected frames without an idle column to be rejected")
	}
}

func TestParseProcStatDiff_OutOfOrderAndCRLF(t *testing.T) {
	// Core order between frames must not matter, and `adb shell` output
	// carries CRLF line endings.
	frameA := "cpu7  0 0 0 800 0 0 0 0\r\ncpu3  0 0 0 800 0 0 0 0\r\ncpu0  0 0 0 800 0 0 0 0\r\n"
	frameB := "cpu0  70 0 0 830 0 0 0 0\r\ncpu3  10 0 0 890 0 0 0 0\r\ncpu7  40 0 0 860 0 0 0 0\r\n"
	overall, cores, ok := parseProcStatDiff(frameA, frameB)
	if !ok {
		t.Fatal("expected out-of-order frames to parse")
	}
	// Contiguous-from-0 only holds for cpu0..cpu7 when all present; here
	// cpu1 is missing so per-core data must be nil, while overall comes
	// from the viable-core mean (70, 10, 40 → 40).
	if cores != nil {
		t.Fatalf("expected nil cores for non-contiguous indices, got %v", cores)
	}
	if diff := overall - 40; diff > 0.01 || diff < -0.01 {
		t.Errorf("overall = %v, want 40", overall)
	}

	// Fully aligned but shuffled order → per-core values stay index-aligned.
	frameA2 := "cpu1  0 0 0 800 0 0 0 0\ncpu0  0 0 0 800 0 0 0 0\n"
	frameB2 := "cpu0  10 0 0 890 0 0 0 0\ncpu1  20 0 0 880 0 0 0 0\n"
	_, cores2, ok := parseProcStatDiff(frameA2, frameB2)
	if !ok {
		t.Fatal("expected shuffled aligned frames to parse")
	}
	if len(cores2) != 2 || cores2[0] != 10 || cores2[1] != 20 {
		t.Fatalf("cores = %v, want [10 20]", cores2)
	}
}

func TestParseProcStatDiff_AggregateFromCoreMean(t *testing.T) {
	// No aggregate "cpu" row at all → overall falls back to the core mean.
	frameA := "cpu0  0 0 0 800 0 0 0 0\ncpu1  0 0 0 800 0 0 0 0\n"
	frameB := "cpu0  10 0 0 890 0 0 0 0\ncpu1  30 0 0 870 0 0 0 0\n"
	overall, cores, ok := parseProcStatDiff(frameA, frameB)
	if !ok {
		t.Fatal("expected frames without aggregate row to parse")
	}
	if len(cores) != 2 || cores[0] != 10 || cores[1] != 30 {
		t.Fatalf("cores = %v, want [10 30]", cores)
	}
	if diff := overall - 20; diff > 0.01 || diff < -0.01 {
		t.Errorf("overall = %v, want 20 (core mean)", overall)
	}
}

func TestParseProcStatDiff_Garbage(t *testing.T) {
	if _, _, ok := parseProcStatDiff("not stat at all\n", "still not stat\n"); ok {
		t.Fatal("expected garbage frames to be rejected")
	}
	// Second frame empty (e.g. second cat failed).
	if _, _, ok := parseProcStatDiff("cpu0  1 2 3 4\n", ""); ok {
		t.Fatal("expected empty second frame to be rejected")
	}
}

func TestSplitProcStat(t *testing.T) {
	out := "cpu  1 2\ncpu0 3 4\r\n" + procStatMarker + "\r\ncpu  5 6\n"
	a, b, ok := splitProcStat(out)
	if !ok {
		t.Fatal("expected marker split")
	}
	if !strings.Contains(a, "cpu0 3 4") || strings.Contains(b, "cpu0") {
		t.Fatalf("split wrong: a=%q b=%q", a, b)
	}
	if _, _, ok := splitProcStat("no marker here"); ok {
		t.Fatal("expected no split without marker")
	}
}
