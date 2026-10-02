package hotkey

import (
	"errors"
	"runtime"
	"testing"
)

func TestParseAndDefault(t *testing.T) {
	acc, err := Parse("Ctrl+Alt+Space")
	if err != nil {
		t.Fatal(err)
	}
	if acc.String() != "ctrl+alt+space" || !acc.Ctrl || !acc.Alt || acc.Key != "space" {
		t.Fatalf("parse: %+v", acc)
	}
	option, err := Parse("option+space")
	if err != nil || option.String() != "alt+space" || !option.Alt {
		t.Fatalf("option: %+v %v", option, err)
	}
	if _, err := Parse("space"); err == nil {
		t.Fatal("bare key must be rejected")
	}
	if _, err := Parse("ctrl+nope"); err == nil {
		t.Fatal("unknown key must be rejected")
	}
	if PlatformDefault() == "" {
		t.Fatal("empty default")
	}
}

func TestListenUnsupportedOnThisOS(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("global hotkey backend exists")
	}
	_, err := Listen("ctrl+alt+space", func() {})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("got %v", err)
	}
}

func TestListenInvalid(t *testing.T) {
	if _, err := Listen("", func() {}); err == nil {
		t.Fatal("empty spec")
	}
	if _, err := Listen("ctrl+alt+space", nil); err == nil {
		t.Fatal("nil callback")
	}
}
