package adb

import "testing"

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

func TestParseNPULoad(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
		ok   bool
	}{
		{"plain percent", "42\n", 42, true},
		{"single core load label", "load: 25%\n", 25, true},
		{"multi core max", "Core 0: 10%\nCore 1: 45%\nCore 2: 0%\n", 45, true},
		{"no percent sign", "37\n", 37, true},
		{"empty", "", 0, false},
		{"garbage", "N/A", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseNPULoad(tc.in)
		if ok != tc.ok {
			t.Fatalf("%s: ok=%v want %v", tc.name, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
