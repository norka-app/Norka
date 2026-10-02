package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStaleRelaunchScripts(t *testing.T) {
	dir := t.TempDir()
	keep := []string{
		"norka-relaunch-abc.cmd",
		"norka-relaunch-.cmd",
		"norka-relaunch-1.sh",
		"notes.txt",
		"norka-relaunch-12.cmd.txt",
	}
	remove := []string{
		"norka-relaunch-1.cmd",
		"norka-relaunch-8125.cmd",
	}
	for _, name := range append(append([]string{}, keep...), remove...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	nestedScript := filepath.Join(nested, "norka-relaunch-3.cmd")
	if err := os.WriteFile(nestedScript, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(target, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "norka-relaunch-5.cmd")
	linkOK := os.Symlink(target, link) == nil

	removeStaleRelaunchScripts(dir)

	for _, name := range remove {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was removed: %v", name, err)
		}
	}
	if _, err := os.Stat(nestedScript); err != nil {
		t.Fatalf("nested script was removed: %v", err)
	}
	if linkOK {
		if _, err := os.Lstat(link); err != nil {
			t.Fatalf("symlink was removed: %v", err)
		}
	}
	body, err := os.ReadFile(target)
	if err != nil || string(body) != "safe" {
		t.Fatalf("symlink target = %q err=%v", body, err)
	}
}
