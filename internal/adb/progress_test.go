package adb

import "testing"

func TestParseProgress(t *testing.T) {
	cases := []struct {
		line      string
		wantBytes int64
		wantTotal int64
		wantOK    bool
	}{
		// adb 37 emits a single summary line per transfer; bytes == total.
		{"/tmp/a.bin: 1 file pushed, 0 skipped. 14.9 MB/s (2000000 bytes in 0.128s)", 2000000, 2000000, true},
		{"/sdcard/b: 1 file pulled, 0 skipped. 8.9 MB/s (2000000 bytes in 0.213s)", 2000000, 2000000, true},
		// Not a transfer summary — must not match.
		{"[  8%] /sdcard/x.jpg", 0, 0, false},
		{"1204/4500 (27%)", 0, 0, false}, // legacy format adb no longer emits
		{"0/0", 0, 0, false},
	}
	for _, c := range cases {
		b, tot, _, ok := ParseProgress(c.line)
		if ok != c.wantOK || b != c.wantBytes || tot != c.wantTotal {
			t.Errorf("ParseProgress(%q) = (%d,%d,%v) want (%d,%d,%v)",
				c.line, b, tot, ok, c.wantBytes, c.wantTotal, c.wantOK)
		}
	}
}

// TestParseProgressRate checks the average speed is read from the summary line,
// normalized to bytes/sec.
func TestParseProgressRate(t *testing.T) {
	// 14.9 MB/s ≈ 15623782 B/s; assert within rounding tolerance of the token.
	_, _, rate, ok := ParseProgress("/x: 1 file pushed, 0 skipped. 14.9 MB/s (2000000 bytes in 0.128s)")
	if !ok {
		t.Fatal("expected match")
	}
	if rate < 14000000 || rate > 16000000 {
		t.Fatalf("rate=%d want ~14.9MB/s", rate)
	}
}

func TestParseDevices(t *testing.T) {
	out := `List of devices attached
emulator-5554   device product:sdk_phone model:Pixel_5 device:emu_trans:0
192.168.1.20:5555  unauthorized product:foo model:Bar trans:tcp
adbserver-123    offline
`
	devs := ParseDevices(out)
	if len(devs) != 3 {
		t.Fatalf("got %d devices", len(devs))
	}
	want0 := [4]string{"emulator-5554", "device", "Pixel_5", "usb"}
	got0 := [4]string{devs[0].Serial, devs[0].State, devs[0].Model, devs[0].Transport}
	if got0 != want0 {
		t.Errorf("device[0] = %v want %v", got0, want0)
	}
	if devs[1].State != "unauthorized" || devs[1].Transport != "tcpip" {
		t.Errorf("device[1] = %+v", devs[1])
	}
	if devs[2].State != "offline" {
		t.Errorf("device[2] state = %q", devs[2].State)
	}
}
