package hotkey

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ErrUnsupported means this OS has no global hotkey backend.
var ErrUnsupported = errors.New("unsupported")

// ErrRegisterFailed means the OS rejected the chord (usually already taken).
var ErrRegisterFailed = errors.New("register_failed")

// ErrInvalid means the accelerator string cannot be parsed.
var ErrInvalid = errors.New("invalid hotkey")

// Accelerator is a keyboard chord. Alt covers Option on macOS.
type Accelerator struct {
	Ctrl  bool
	Alt   bool
	Shift bool
	Meta  bool
	Key   string
}

// PlatformDefault is Ctrl+Alt+Space on Windows and Option+Space on macOS.
// Other systems use the Windows chord; registration still reports unsupported.
func PlatformDefault() string {
	if runtime.GOOS == "darwin" {
		return "option+space"
	}
	return "ctrl+alt+space"
}

// Parse reads chords like "ctrl+alt+space" or "option+space".
// Modifier and key names are case-insensitive. "option" is Alt.
func Parse(spec string) (Accelerator, error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" {
		return Accelerator{}, fmt.Errorf("%w: empty", ErrInvalid)
	}
	parts := strings.FieldsFunc(spec, func(r rune) bool {
		return r == '+' || r == '-' || r == ' '
	})
	if len(parts) == 0 {
		return Accelerator{}, fmt.Errorf("%w: empty", ErrInvalid)
	}
	var acc Accelerator
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i < len(parts)-1 {
			switch part {
			case "ctrl", "control":
				acc.Ctrl = true
			case "alt", "option", "opt":
				acc.Alt = true
			case "shift":
				acc.Shift = true
			case "meta", "cmd", "command", "super", "win":
				acc.Meta = true
			default:
				return Accelerator{}, fmt.Errorf("%w: unknown modifier %q", ErrInvalid, part)
			}
			continue
		}
		key, ok := normalizeKey(part)
		if !ok {
			return Accelerator{}, fmt.Errorf("%w: unknown key %q", ErrInvalid, part)
		}
		acc.Key = key
	}
	if acc.Key == "" {
		return Accelerator{}, fmt.Errorf("%w: missing key", ErrInvalid)
	}
	if !acc.Ctrl && !acc.Alt && !acc.Shift && !acc.Meta {
		return Accelerator{}, fmt.Errorf("%w: need a modifier", ErrInvalid)
	}
	return acc, nil
}

// String returns a canonical chord. Alt is written as "alt".
func (a Accelerator) String() string {
	var parts []string
	if a.Ctrl {
		parts = append(parts, "ctrl")
	}
	if a.Alt {
		parts = append(parts, "alt")
	}
	if a.Shift {
		parts = append(parts, "shift")
	}
	if a.Meta {
		parts = append(parts, "meta")
	}
	if a.Key != "" {
		parts = append(parts, a.Key)
	}
	return strings.Join(parts, "+")
}

func normalizeKey(part string) (string, bool) {
	switch part {
	case "space", "spacebar":
		return "space", true
	case "esc", "escape":
		return "escape", true
	case "tab":
		return "tab", true
	case "enter", "return":
		return "enter", true
	}
	if len(part) == 1 {
		r := part[0]
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return part, true
		}
	}
	if len(part) >= 2 && (part[0] == 'f') {
		n := 0
		for _, r := range part[1:] {
			if r < '0' || r > '9' {
				return "", false
			}
			n = n*10 + int(r-'0')
		}
		if n >= 1 && n <= 12 {
			return part, true
		}
	}
	return "", false
}

// Listen registers a global hotkey until the returned function is called.
// The callback is invoked on a private goroutine and must not block for long.
func Listen(spec string, callback func()) (func(), error) {
	if callback == nil {
		return nil, fmt.Errorf("%w: nil callback", ErrInvalid)
	}
	acc, err := Parse(spec)
	if err != nil {
		return nil, err
	}
	return listen(acc, callback)
}
