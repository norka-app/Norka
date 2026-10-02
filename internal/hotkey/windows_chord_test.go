package hotkey

import (
	"errors"
	"testing"
)

func TestWinChord(t *testing.T) {
	mods, vk, err := winChord(Accelerator{Ctrl: true, Alt: true, Key: "space"})
	if err != nil {
		t.Fatal(err)
	}
	if mods != modCtrl|modAlt || vk != 0x20 {
		t.Fatalf("space: mods=%#x vk=%#x", mods, vk)
	}
	mods, vk, err = winChord(Accelerator{Ctrl: true, Key: "k"})
	if err != nil || mods != modCtrl || vk != 0x4B {
		t.Fatalf("k: mods=%#x vk=%#x err=%v", mods, vk, err)
	}
	mods, vk, err = winChord(Accelerator{Meta: true, Shift: true, Key: "f12"})
	if err != nil || mods != modWin|modShift || vk != 0x7B {
		t.Fatalf("f12: mods=%#x vk=%#x err=%v", mods, vk, err)
	}
	_, _, err = winChord(Accelerator{Ctrl: true, Key: "л"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("cyrillic key: %v", err)
	}
}
