package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	const sum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	text := "# comment\n" +
		sum + "  norka.exe\n" +
		sum + " *windows/norka.dmg\n" +
		"SHA256 (SHA256SUMS) = " + sum + "\n" +
		"not-a-sum  file\n"
	got := parseChecksums(text)
	if got["norka.exe"] != sum {
		t.Fatalf("exe = %q", got["norka.exe"])
	}
	if got["norka.dmg"] != sum {
		t.Fatalf("dmg = %q", got["norka.dmg"])
	}
	if got["sha256sums"] != sum {
		t.Fatalf("bsd = %q", got["sha256sums"])
	}
}

func TestDigestForAssetPrefersAPI(t *testing.T) {
	const api = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const file = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	got := digestForAsset("norka.exe", "sha256:"+api, file+"  norka.exe\n")
	if got != api {
		t.Fatalf("digest = %s", got)
	}
	got = digestForAsset("norka.exe", "", file+"  norka.exe\n")
	if got != file {
		t.Fatalf("file digest = %s", got)
	}
	if digestForAsset("other.exe", "", file+"  norka.exe\n") != "" {
		t.Fatal("unexpected digest")
	}
	if parseDigest("sha512:"+api) != "" {
		t.Fatal("non-sha256 digest should be ignored")
	}
}

func TestVerifyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "norka.exe")
	payload := []byte("norka-update")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256Hex(payload)
	if err := verifyFile(path, int64(len(payload)), sum); err != nil {
		t.Fatal(err)
	}
	if err := verifyFile(path, int64(len(payload)+1), sum); err != errSize {
		t.Fatalf("size err = %v", err)
	}
	if err := verifyFile(path, int64(len(payload)), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if err := verifyFile(path, int64(len(payload)), "nope"); err == nil {
		t.Fatal("expected missing digest to fail")
	}
}
