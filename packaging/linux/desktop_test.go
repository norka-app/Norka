package desktop_test

import (
	"os"
	"strings"
	"testing"
)

func TestDesktopDeclaresNorkaScheme(t *testing.T) {
	desktop, err := os.ReadFile("norka.desktop")
	if err != nil {
		t.Fatal(err)
	}
	text := string(desktop)
	if !strings.Contains(text, "MimeType=x-scheme-handler/norka;") {
		t.Fatalf("desktop file has no norka scheme:\n%s", text)
	}
	if !strings.Contains(text, "%u") {
		t.Fatal("desktop Exec does not receive the URL")
	}
	script, err := os.ReadFile("package.sh")
	if err != nil {
		t.Fatal(err)
	}
	body := string(script)
	for _, want := range []string{"Exec=norka %u", "Exec=/usr/bin/norka %u", "x-scheme-handler/norka"} {
		if !strings.Contains(body, want) {
			t.Fatalf("package.sh missing %s", want)
		}
	}
	if strings.Count(body, "%u") < 3 {
		t.Fatal("AppImage, deb and tar.gz must all pass the URL")
	}
}

func TestWailsDeclaresNorkaProtocol(t *testing.T) {
	raw, err := os.ReadFile("../../wails.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"scheme": "norka"`) {
		t.Fatal("wails.json does not register the norka URL scheme")
	}
}
