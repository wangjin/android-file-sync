package localfs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	// a file
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// a subdir
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := ListDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries", len(entries))
	}

	byName := map[string]bool{}
	for _, e := range entries {
		byName[e.Name] = true
		if e.Path == "" {
			t.Error("empty path")
		}
	}
	if !byName["a.txt"] || !byName["sub"] {
		t.Errorf("missing entries: %v", byName)
	}

	for _, e := range entries {
		if e.Name == "a.txt" && (e.IsDir || e.Size != 5) {
			t.Errorf("a.txt wrong: %+v", e)
		}
		if e.Name == "sub" && !e.IsDir {
			t.Errorf("sub not dir: %+v", e)
		}
	}
}

func TestListDirNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := ListDir(missing)
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestParentDir(t *testing.T) {
	cases := map[string]string{
		"/a/b/c":  "/a/b",
		"/a":      "/",
		"/":       "/",
		"a/b":     "a",
	}
	if runtime.GOOS == "windows" {
		// windows path separators handled by filepath; skip unix-only cases
		return
	}
	for in, want := range cases {
		if got := ParentDir(in); got != want {
			t.Errorf("ParentDir(%q) = %q want %q", in, got, want)
		}
	}
}
