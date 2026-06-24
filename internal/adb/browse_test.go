package adb

import "testing"

func TestParseListing(t *testing.T) {
	const out = `total 48
drwxrwx--x 4 root sdcard_rw 4096 2026-06-20 13:01 DCIM
-rw-rw---- 1 root sdcard_rw  1024 2026-06-20 13:02 notes.txt
lrw-rw-rw- 1 root sdcard_rw    12 2026-06-20 13:03 link -> target.txt
drwxr-xr-x 2 root root        4096 2026-06-20 13:04 我的文件夹
`
	entries, err := ParseListing("/sdcard", out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries", len(entries))
	}
	dcim := entries[0]
	if dcim.Name != "DCIM" || !dcim.IsDir || dcim.Size != 4096 || dcim.Path != "/sdcard/DCIM" {
		t.Errorf("DCIM wrong: %+v", dcim)
	}
	notes := entries[1]
	if notes.IsDir || notes.Size != 1024 {
		t.Errorf("notes wrong: %+v", notes)
	}
	link := entries[2]
	if link.Link != "target.txt" {
		t.Errorf("link target wrong: %+v", link)
	}
	cn := entries[3]
	if cn.Name != "我的文件夹" || !cn.IsDir {
		t.Errorf("cn folder wrong: %+v", cn)
	}
}

func TestParseListingPermissionDenied(t *testing.T) {
	const out = `total 0
adb: permission denied
`
	entries, err := ParseListing("/data", out)
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if entries != nil {
		t.Fatal("expected nil entries")
	}
}
