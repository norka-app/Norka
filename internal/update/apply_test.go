package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwapFile(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "norka.exe")
	next := filepath.Join(dir, "next.exe")
	if err := os.WriteFile(current, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := swapFile(current, next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("current = %q", got)
	}
	old, err := os.ReadFile(current + ".old")
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != "old" {
		t.Fatalf("backup = %q", old)
	}
}

func TestBundleFromExeAndFindApp(t *testing.T) {
	bundle, ok := bundleFromExe(`/Applications/norka.app/Contents/MacOS/norka`)
	if !ok || bundle != `/Applications/norka.app` {
		t.Fatalf("bundle = %q ok=%v", bundle, ok)
	}
	if _, ok := bundleFromExe(`/tmp/norka`); ok {
		t.Fatal("plain binary should not look like a bundle")
	}

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Other.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "norka.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := findAppBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(found) != "norka.app" {
		t.Fatalf("found = %s", found)
	}
}

func TestParseMountPoint(t *testing.T) {
	plist := []byte(`<key>mount-point</key>
<string>/Volumes/norka</string>`)
	got, err := parseMountPoint(plist)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/Volumes/norka" {
		t.Fatalf("mount = %s", got)
	}
	if _, err := parseMountPoint([]byte("<plist></plist>")); err == nil {
		t.Fatal("expected error")
	}
}

func TestRelaunchScripts(t *testing.T) {
	sh := darwinRelaunchScript(99, "/Applications/Norka App.app")
	if !strings.Contains(sh, "PID=99") || !strings.Contains(sh, "'/Applications/Norka App.app'") {
		t.Fatalf("darwin script = %s", sh)
	}
	quoted := darwinRelaunchScript(1, "/tmp/it's.app")
	if !strings.Contains(quoted, `'/tmp/it'\''s.app'`) {
		t.Fatalf("quote = %s", quoted)
	}
}

func TestApplyDoesNotInstallOnThisOS(t *testing.T) {
	offer := Offer{
		Available: true,
		URL:       "https://github.com/norka-app/Norka/releases/download/v1/norka.exe",
		CanApply:  true,
		Digest:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AssetName: "norka.exe",
	}
	result, err := Apply(t.Context(), offer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fallback || result.Code != CodeNotWritable || result.Restarting {
		t.Fatalf("result = %+v", result)
	}

	link := Offer{Available: true, URL: "https://github.com/norka-app/Norka/releases/tag/v1", CanApply: false}
	result, err = Apply(t.Context(), link, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fallback || result.Code != CodeUnsupported {
		t.Fatalf("link result = %+v", result)
	}
}

func TestDirWritableAndSkipFile(t *testing.T) {
	dir := t.TempDir()
	if !dirWritable(dir) {
		t.Fatal("temp dir should be writable")
	}
	if dirWritable(filepath.Join(dir, "missing")) {
		t.Fatal("missing dir should not be writable")
	}
	if ReadSkip(dir) != "" {
		t.Fatal("empty skip")
	}
	if err := WriteSkip(dir, "v1.2.0"); err != nil {
		t.Fatal(err)
	}
	if ReadSkip(dir) != "1.2.0" {
		t.Fatalf("skip = %q", ReadSkip(dir))
	}
	if err := WriteSkip("", "1.0.0"); err == nil {
		t.Fatal("expected error for empty dir")
	}
}
