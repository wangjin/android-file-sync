package adb

import "testing"

func TestParseProgress(t *testing.T) {
	cases := []struct {
		line      string
		wantBytes int64
		wantTotal int64
		wantOK    bool
	}{
		{"[  8%] /sdcard/x.jpg", 0, 0, false},          // not a push/pull progress line
		{"1 file pulled, 0 skipped. 3.2 MB/s (4500 bytes in 0.001s)", 0, 0, false},
		{"1204/4500 (27%)", 1204, 4500, true},
		{"4500/4500 (100%)", 4500, 4500, true},
		{"0/0", 0, 0, false},
	}
	for _, c := range cases {
		b, tot, ok := ParseProgress(c.line)
		if ok != c.wantOK || b != c.wantBytes || tot != c.wantTotal {
			t.Errorf("ParseProgress(%q) = (%d,%d,%v) want (%d,%d,%v)",
				c.line, b, tot, ok, c.wantBytes, c.wantTotal, c.wantOK)
		}
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
