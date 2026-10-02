package uilocale

import "testing"

func TestClassifyLocale(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "", want: ""},
		{raw: "C", want: ""},
		{raw: "POSIX", want: ""},
		{raw: "C.UTF-8", want: ""},
		{raw: "ru", want: "ru"},
		{raw: "ru_RU.UTF-8", want: "ru"},
		{raw: "ru-RU", want: "ru"},
		{raw: "en_US.UTF-8", want: "en"},
		{raw: "en-GB", want: "en"},
		{raw: "zh_CN.UTF-8", want: "en"},
	}
	for _, tc := range cases {
		if got := classifyLocale(tc.raw); got != tc.want {
			t.Errorf("classifyLocale(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestNormalizePreference(t *testing.T) {
	if got := NormalizePreference("  RU_ru.UTF-8 "); got != PrefRU {
		t.Fatalf("ru tag = %q", got)
	}
	if got := NormalizePreference("en-GB"); got != PrefEN {
		t.Fatalf("en tag = %q", got)
	}
	if got := NormalizePreference("french"); got != PrefAuto {
		t.Fatalf("unknown = %q, want auto", got)
	}
	if got := NormalizePreference(""); got != PrefAuto {
		t.Fatalf("empty = %q, want auto", got)
	}
}

func TestEffectiveFollowsEnv(t *testing.T) {
	t.Setenv("LC_ALL", "ru_RU.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	if got := Effective("auto"); got != "ru" {
		t.Fatalf("auto ru = %q", got)
	}
	if got := Effective(""); got != "ru" {
		t.Fatalf("empty ru = %q", got)
	}
	if got := Effective("en"); got != "en" {
		t.Fatalf("pinned en = %q", got)
	}

	t.Setenv("LC_ALL", "en_US.UTF-8")
	if got := Effective("auto"); got != "en" {
		t.Fatalf("auto en = %q", got)
	}
	if got := Effective("ru"); got != "ru" {
		t.Fatalf("pinned ru = %q", got)
	}
	if got := DetectFromEnv(); got != "en" {
		t.Fatalf("detect = %q", got)
	}
}

func TestWriteFileStoresPreference(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFile(dir, "en-US"); err != nil {
		t.Fatal(err)
	}
	if got := ReadFile(dir); got != PrefEN {
		t.Fatalf("file = %q", got)
	}
	if got := Resolve(dir); got != PrefEN {
		t.Fatalf("resolve en = %q", got)
	}

	t.Setenv("LC_ALL", "ru_RU.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	if err := WriteFile(dir, "auto"); err != nil {
		t.Fatal(err)
	}
	if got := ReadFile(dir); got != PrefAuto {
		t.Fatalf("file = %q, want auto", got)
	}
	if got := Resolve(dir); got != PrefRU {
		t.Fatalf("resolve auto = %q, want ru", got)
	}
}
