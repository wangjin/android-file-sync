package localfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remove(target); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("file still exists")
	}
}

func TestRemoveDirRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remove(sub); err != nil {
		t.Fatalf("Remove dir: %v", err)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Fatal("dir still exists")
	}
}

func TestRemoveMissing(t *testing.T) {
	dir := t.TempDir()
	// os.RemoveAll treats a missing path as success (idempotent): the desired
	// end state — path gone — already holds. We keep that contract so delete
	// is safe to retry.
	if err := Remove(filepath.Join(dir, "nope")); err != nil {
		t.Fatalf("Remove of missing path should be nil, got %v", err)
	}
}
