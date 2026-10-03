package filelock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileLockRewriteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.lock")
	lk, err := Acquire(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("payload\n")
	if err := lk.Rewrite(body); err != nil {
		t.Fatal(err)
	}
	if err := lk.Release(); err != nil {
		t.Fatal(err)
	}
	if err := lk.Release(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("file = %q", got)
	}
	again, err := Acquire(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Release() })
}
